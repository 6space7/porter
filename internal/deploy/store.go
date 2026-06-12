package deploy

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"fmt"

	"github.com/6space7/porter/internal/store"
)

type deploymentIDFunc func() string

type storeDeploymentStore struct {
	queries *store.Queries
	newID   deploymentIDFunc
}

func NewStoreDeploymentStore(queries *store.Queries, newID deploymentIDFunc) DeploymentStore {
	if newID == nil {
		newID = func() string {
			return randomDeploymentID()
		}
	}
	return storeDeploymentStore{queries: queries, newID: newID}
}

func (deploymentStore storeDeploymentStore) CreateDeployment(ctx context.Context, appID string) (DeploymentRecord, error) {
	row, err := deploymentStore.queries.CreateDeployment(ctx, store.CreateDeploymentParams{
		ID:       deploymentStore.newID(),
		AppID:    appID,
		Status:   string(StatusRunning),
		Stage:    string(StageQueued),
		BuildLog: "",
		ImageTag: sql.NullString{},
	})
	if err != nil {
		return DeploymentRecord{}, err
	}
	return deploymentRecord(row), nil
}

func (deploymentStore storeDeploymentStore) UpdateDeployment(ctx context.Context, record DeploymentRecord) error {
	return deploymentStore.queries.UpdateDeploymentStatus(ctx, store.UpdateDeploymentStatusParams{
		ID:       record.ID,
		Status:   string(record.Status),
		Stage:    string(record.Stage),
		BuildLog: record.BuildLog,
		ImageTag: sql.NullString{
			String: record.ImageTag,
			Valid:  record.ImageTag != "",
		},
	})
}

func deploymentRecord(row store.Deployment) DeploymentRecord {
	imageTag := ""
	if row.ImageTag.Valid {
		imageTag = row.ImageTag.String
	}
	return DeploymentRecord{
		ID:       row.ID,
		AppID:    row.AppID,
		Status:   Status(row.Status),
		Stage:    Stage(row.Stage),
		BuildLog: row.BuildLog,
		ImageTag: imageTag,
	}
}

func randomDeploymentID() string {
	var raw [16]byte
	if _, err := rand.Read(raw[:]); err != nil {
		panic(fmt.Sprintf("generate deployment id: %v", err))
	}
	return "dep_" + base64.RawURLEncoding.EncodeToString(raw[:])
}
