package graph

// Hand-written helpers for the issue-09 blessed content mutations. This lives in
// a non-`*.resolvers.go` file on purpose: gqlgen's copy-through only preserves
// methods that implement a resolver interface, and would otherwise strip a helper
// method out of the resolver file on every `sqlgen generate`.

import (
	"context"
	"errors"
	"fmt"

	"github.com/teandresmith/sqlgen-example/internal/auth"
	"github.com/teandresmith/sqlgen-example/internal/database"
	"github.com/teandresmith/sqlgen-example/internal/graph/sqlgenresolver"
	"uuid"
)

// createAttachment is the shared write path for attachToTask / attachToComment.
// The caller has already resolved the target tenant-scoped; this stamps the
// uploader from the bearer token, sets the polymorphic binding the server owns,
// and lets tenant scoping inject workspace_id.
func (r *mutationResolver) createAttachment(ctx context.Context, entityType database.AttachmentEntityType, entityID uuid.UUID, fileName string, contentType string, sizeBytes int, storageURL string) (*database.Attachment, error) {
	uploadedBy, ok := auth.UserID(ctx)
	if !ok {
		return nil, errors.New("unauthenticated: a valid bearer token is required to attach a file")
	}

	attachment, err := r.Client.Attachments().Create(ctx, &database.CreateAttachmentInput{
		EntityType:  entityType,
		EntityID:    entityID,
		UploadedBy:  uploadedBy,
		FileName:    fileName,
		ContentType: contentType,
		SizeBytes:   int64(sizeBytes),
		StorageURL:  storageURL,
	}, func(o *database.CallOptions[database.AttachmentFieldOptions]) {
		for _, apply := range sqlgenresolver.CallOptionsFromHTTP[database.AttachmentFieldOptions](ctx) {
			apply(o)
		}
	})
	if err != nil {
		return nil, fmt.Errorf("attaching file: %w", err)
	}
	return attachment, nil
}
