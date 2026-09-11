package test

import (
	"testing"
	"time"
)

// ---- membership-administration helpers (issue 04) -------------------------

const inviteMutation = `
	mutation ($input: InviteToWorkspaceInput!) {
		inviteToWorkspace(input: $input) { id email role status token workspaceID }
	}`

type invitation struct {
	ID          string `json:"id"`
	Email       string `json:"email"`
	Role        string `json:"role"`
	Status      string `json:"status"`
	Token       string `json:"token"`
	WorkspaceID string `json:"workspaceID"`
}

// invite has the owner/admin holding token invite email at role into the Active
// Workspace wsID, returning the created Invitation (including its token).
func invite(t *testing.T, token, wsID, email, role string) invitation {
	t.Helper()
	var out struct {
		InviteToWorkspace invitation `json:"inviteToWorkspace"`
	}
	gqlExecData(t, inviteMutation, map[string]any{
		"input": map[string]any{"email": email, "role": role},
	}, workspaceHeaders(token, wsID), &out)
	return out.InviteToWorkspace
}

const acceptMutation = `
	mutation ($token: String!) {
		acceptInvitation(token: $token) { workspaceID userID role }
	}`

// acceptData accepts an invitation as the caller (bearer only, no Active
// Workspace) and returns the raw response so the caller can assert success or an
// expected error.
func acceptData(t *testing.T, token, inviteToken string) gqlResponse {
	t.Helper()
	return gqlExec(t, acceptMutation, map[string]any{"token": inviteToken}, authHeaders(token))
}

const revokeMutation = `
	mutation ($id: UUID!) {
		revokeInvitation(id: $id) { id status }
	}`

// invitationStatus reads an Invitation's current status from the Active
// Workspace (owner/admin view).
func invitationStatus(t *testing.T, token, wsID, id string) string {
	t.Helper()
	const q = `query ($id: UUID!) { invitation(id: $id) { status } }`
	var out struct {
		Invitation *struct {
			Status string `json:"status"`
		} `json:"invitation"`
	}
	gqlExecData(t, q, map[string]any{"id": id}, workspaceHeaders(token, wsID), &out)
	if out.Invitation == nil {
		t.Fatalf("invitation %s not visible in workspace %s", id, wsID)
	}
	return out.Invitation.Status
}

// ---- tests ----------------------------------------------------------------

// TestOwnerInvitesAndInviteeAccepts proves the happy path of stories 5 and 6:
// an owner invites a person by email with a Role, and that person accepts,
// which creates their Membership (at the invited Role) and marks the Invitation
// `accepted`. Acceptance is proven behaviorally — after accepting, the invitee
// can select the Workspace (only a Member can) and reads back their Membership.
func TestOwnerInvitesAndInviteeAccepts(t *testing.T) {
	ownerToken, _ := register(t, "inv-owner@example.com", "owner-invites-1", "Owner")
	wsID := bootstrapWorkspace(t, ownerToken, "Invites Co", "invites-co")

	inv := invite(t, ownerToken, wsID, "invitee@example.com", "MEMBER")
	if inv.Status != "PENDING" {
		t.Fatalf("new invitation status = %q, want PENDING", inv.Status)
	}
	if inv.Token == "" {
		t.Fatal("invitation token is empty; invitee has nothing to accept with")
	}
	if inv.WorkspaceID != wsID {
		t.Errorf("invitation workspaceID = %q, want %q (auto-scoped to Active Workspace)", inv.WorkspaceID, wsID)
	}

	// The invited person registers (their email matches the invitation) and accepts.
	inviteeToken, invitee := register(t, "invitee@example.com", "accept-me-123", "Invitee")
	resp := acceptData(t, inviteeToken, inv.Token)
	if len(resp.Errors) > 0 {
		t.Fatalf("accept invitation failed: %+v", resp.Errors)
	}
	var accepted struct {
		AcceptInvitation struct {
			WorkspaceID string `json:"workspaceID"`
			UserID      string `json:"userID"`
			Role        string `json:"role"`
		} `json:"acceptInvitation"`
	}
	decode(t, resp, &accepted)
	if accepted.AcceptInvitation.WorkspaceID != wsID || accepted.AcceptInvitation.UserID != invitee.ID {
		t.Errorf("new membership pk = (%s, %s), want (%s, %s)",
			accepted.AcceptInvitation.WorkspaceID, accepted.AcceptInvitation.UserID, wsID, invitee.ID)
	}
	if accepted.AcceptInvitation.Role != "MEMBER" {
		t.Errorf("new membership role = %q, want MEMBER (the invited Role)", accepted.AcceptInvitation.Role)
	}

	// The Invitation is now accepted. The status update's cache eviction is
	// post-commit async (issue 05), so poll with a bounded timeout — the same
	// pattern the cache suite uses for this eviction — instead of reading once.
	deadline := time.Now().Add(2 * time.Second)
	var status string
	for time.Now().Before(deadline) {
		if status = invitationStatus(t, ownerToken, wsID, inv.ID); status == "ACCEPTED" {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if status != "ACCEPTED" {
		t.Errorf("invitation status after accept = %q, want ACCEPTED within 2s", status)
	}

	// ...and the invitee is a real Member: they can select the Workspace and
	// read their own Membership back (the middleware only admits Members).
	const meMembership = `query ($ws: UUID!, $u: UUID!) { membership(workspaceID: $ws, userID: $u) { role } }`
	var m struct {
		Membership *struct {
			Role string `json:"role"`
		} `json:"membership"`
	}
	gqlExecData(t, meMembership, map[string]any{"ws": wsID, "u": invitee.ID}, workspaceHeaders(inviteeToken, wsID), &m)
	if m.Membership == nil || m.Membership.Role != "MEMBER" {
		t.Errorf("invitee membership = %+v, want role MEMBER", m.Membership)
	}
}

// TestAcceptWithForeignWorkspaceSelected guards a subtle authz-hook coupling:
// acceptInvitation mints a Membership in the invited Workspace via a self-
// authorized (SkipTenancy) write, but the acceptor may already belong to some
// other Workspace and have it selected via X-Workspace-Id. The authz hook must
// key on that write being self-authorized, not on the acceptor's ambient Role
// in the selected Workspace — otherwise a plain member of Workspace A could not
// accept an invitation to Workspace B. Here the acceptor accepts while A is
// their Active Workspace and the accept must still succeed.
func TestAcceptWithForeignWorkspaceSelected(t *testing.T) {
	// Bob becomes a plain member of Workspace A.
	ownerA, _ := register(t, "foreign-owner-a@example.com", "owner-a-pw-1", "OwnerA")
	wsA := bootstrapWorkspace(t, ownerA, "Foreign A", "foreign-a")
	invA := invite(t, ownerA, wsA, "foreign-bob@example.com", "MEMBER")
	bobToken, bob := register(t, "foreign-bob@example.com", "bob-foreign-1", "ForeignBob")
	if resp := acceptData(t, bobToken, invA.Token); len(resp.Errors) > 0 {
		t.Fatalf("bob accept into A failed: %+v", resp.Errors)
	}

	// A different owner invites Bob to Workspace B.
	ownerB, _ := register(t, "foreign-owner-b@example.com", "owner-b-pw-1", "OwnerB")
	wsB := bootstrapWorkspace(t, ownerB, "Foreign B", "foreign-b")
	invB := invite(t, ownerB, wsB, "foreign-bob@example.com", "MEMBER")

	// Bob accepts the B invitation while A (where he is only a member) is his
	// Active Workspace. The self-authorized membership write must not be judged
	// against his member Role in A.
	resp := gqlExec(t, acceptMutation, map[string]any{"token": invB.Token}, workspaceHeaders(bobToken, wsA))
	if len(resp.Errors) > 0 {
		t.Fatalf("accept into B with A selected failed: %+v", resp.Errors)
	}
	var accepted struct {
		AcceptInvitation struct {
			WorkspaceID string `json:"workspaceID"`
			UserID      string `json:"userID"`
		} `json:"acceptInvitation"`
	}
	decode(t, resp, &accepted)
	if accepted.AcceptInvitation.WorkspaceID != wsB || accepted.AcceptInvitation.UserID != bob.ID {
		t.Errorf("new membership pk = (%s, %s), want (%s, %s)",
			accepted.AcceptInvitation.WorkspaceID, accepted.AcceptInvitation.UserID, wsB, bob.ID)
	}
}

// TestRevokedInvitationCannotBeAccepted proves story 7: an owner revokes a
// pending Invitation, after which it can no longer be accepted. The would-be
// invitee holds the token but acceptance is refused, and no Membership appears.
func TestRevokedInvitationCannotBeAccepted(t *testing.T) {
	ownerToken, _ := register(t, "rev-owner@example.com", "owner-revokes-1", "RevOwner")
	wsID := bootstrapWorkspace(t, ownerToken, "Revoke Co", "revoke-co")

	inv := invite(t, ownerToken, wsID, "revoked@example.com", "MEMBER")

	// Owner revokes the pending invitation.
	var revoked struct {
		RevokeInvitation struct {
			Status string `json:"status"`
		} `json:"revokeInvitation"`
	}
	gqlExecData(t, revokeMutation, map[string]any{"id": inv.ID}, workspaceHeaders(ownerToken, wsID), &revoked)
	if revoked.RevokeInvitation.Status != "REVOKED" {
		t.Fatalf("revoked invitation status = %q, want REVOKED", revoked.RevokeInvitation.Status)
	}

	// The invitee registers and tries to accept — rejected because the
	// invitation is no longer pending.
	inviteeToken, _ := register(t, "revoked@example.com", "too-late-123", "TooLate")
	resp := acceptData(t, inviteeToken, inv.Token)
	if len(resp.Errors) == 0 {
		t.Fatal("accepting a revoked invitation succeeded, want an error")
	}

	// And no Membership was minted: the invitee cannot select the Workspace.
	status := postStatus(t, `{ __typename }`, workspaceHeaders(inviteeToken, wsID))
	if status != 403 {
		t.Errorf("HTTP status selecting workspace after refused accept = %d, want 403 (never became a member)", status)
	}
}

// TestAdminManagesMembershipsAndRoles proves story 11: a Workspace admin can
// manage Memberships and Roles. An admin (not the owner) re-grades another
// Member's Role via the generated updateMembership, which the authz hook admits
// because admin is a manager Role.
func TestAdminManagesMembershipsAndRoles(t *testing.T) {
	ownerToken, _ := register(t, "adm-owner@example.com", "owner-admin-1", "AdmOwner")
	wsID := bootstrapWorkspace(t, ownerToken, "Admin Co", "admin-co")

	// Owner invites an admin and a plain member; both accept.
	adminInv := invite(t, ownerToken, wsID, "the-admin@example.com", "ADMIN")
	adminToken, _ := register(t, "the-admin@example.com", "admin-pw-123", "TheAdmin")
	if resp := acceptData(t, adminToken, adminInv.Token); len(resp.Errors) > 0 {
		t.Fatalf("admin accept failed: %+v", resp.Errors)
	}

	memberInv := invite(t, ownerToken, wsID, "plain-member@example.com", "MEMBER")
	memberToken, memberUser := register(t, "plain-member@example.com", "member-pw-123", "PlainMember")
	if resp := acceptData(t, memberToken, memberInv.Token); len(resp.Errors) > 0 {
		t.Fatalf("member accept failed: %+v", resp.Errors)
	}

	// The admin promotes the member to admin.
	const updateMembership = `
		mutation ($ws: UUID!, $u: UUID!, $input: UpdateMembershipInput!) {
			updateMembership(workspaceID: $ws, userID: $u, input: $input) { role }
		}`
	var updated struct {
		UpdateMembership struct {
			Role string `json:"role"`
		} `json:"updateMembership"`
	}
	gqlExecData(t, updateMembership, map[string]any{
		"ws":    wsID,
		"u":     memberUser.ID,
		"input": map[string]any{"role": "ADMIN"},
	}, workspaceHeaders(adminToken, wsID), &updated)
	if updated.UpdateMembership.Role != "ADMIN" {
		t.Errorf("member role after admin update = %q, want ADMIN", updated.UpdateMembership.Role)
	}
}

// TestInsufficientRoleRejected is the acceptance test the ticket calls for: an
// insufficient-Role caller is rejected for an owner/admin-only operation (AC 6,
// story 50). A plain Member of a Workspace may read and create work items, but
// the authz hook rejects them when they try to invite (an owner/admin-only
// operation) or re-grade a Membership. The owner performing the same invite is
// the control, proving it is the Role — not the operation — that is refused.
func TestInsufficientRoleRejected(t *testing.T) {
	ownerToken, ownerUser := register(t, "guard-owner@example.com", "owner-guard-1", "GuardOwner")
	wsID := bootstrapWorkspace(t, ownerToken, "Guarded Co", "guarded-co")

	// A plain member joins.
	memberInv := invite(t, ownerToken, wsID, "just-a-member@example.com", "MEMBER")
	memberToken, _ := register(t, "just-a-member@example.com", "member-guard-1", "JustAMember")
	if resp := acceptData(t, memberToken, memberInv.Token); len(resp.Errors) > 0 {
		t.Fatalf("member accept failed: %+v", resp.Errors)
	}

	// The member tries to invite someone — forbidden by the authz hook.
	inviteResp := gqlExec(t, inviteMutation, map[string]any{
		"input": map[string]any{"email": "sneaky@example.com", "role": "MEMBER"},
	}, workspaceHeaders(memberToken, wsID))
	if len(inviteResp.Errors) == 0 {
		t.Error("a member was allowed to invite; owner/admin-only operation not guarded")
	}

	// The member tries to re-grade the owner's Membership — also forbidden.
	const updateMembership = `
		mutation ($ws: UUID!, $u: UUID!, $input: UpdateMembershipInput!) {
			updateMembership(workspaceID: $ws, userID: $u, input: $input) { role }
		}`
	updateResp := gqlExec(t, updateMembership, map[string]any{
		"ws":    wsID,
		"u":     ownerUser.ID,
		"input": map[string]any{"role": "GUEST"},
	}, workspaceHeaders(memberToken, wsID))
	if len(updateResp.Errors) == 0 {
		t.Error("a member was allowed to update a membership; owner/admin-only operation not guarded")
	}

	// Control: the owner CAN invite, so the rejection above is about Role, not
	// a broken operation.
	if resp := gqlExec(t, inviteMutation, map[string]any{
		"input": map[string]any{"email": "welcome@example.com", "role": "MEMBER"},
	}, workspaceHeaders(ownerToken, wsID)); len(resp.Errors) > 0 {
		t.Errorf("owner invite failed, control broken: %+v", resp.Errors)
	}
}
