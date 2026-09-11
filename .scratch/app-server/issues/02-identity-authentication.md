# 02 — Identity & authentication

Status: done
Blocked by: 01-walking-skeleton

Parent: `.scratch/app-server/PRD.md`

## What to build

Global User identity and JWT authentication. A person can register with an email and password (stored as a hash); a `login` mutation verifies the credentials and issues a JWT that carries the User identity only — not bound to any Workspace (per ADR-0004). A stdlib `net/http` middleware validates the bearer JWT on each request and places the authenticated User into the request context. Because `users` are shared/untenanted, register and login work before any tenant is established. Add a test helper that mints JWTs through the same signing code the server uses, so downstream tickets can authenticate for real.

## Acceptance criteria

- [ ] A person can register with email + password; the password is stored hashed (PRD story 1)
- [ ] A `login` mutation verifies email + password hash and returns a JWT (stories 2)
- [ ] The JWT carries the User identity only and binds the caller to no Workspace (story 3)
- [ ] Bearer-JWT middleware validates the token and puts the User into request context; invalid/missing tokens are rejected for authenticated operations
- [ ] A test helper mints JWTs via the server's own signing code
- [ ] Integration test proves register → login → authenticated request end to end

## Blocked by

- 01-walking-skeleton
