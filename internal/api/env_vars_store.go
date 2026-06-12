package api

import (
	"context"
	"fmt"

	secretcrypto "github.com/6space7/porter/internal/crypto"
	"github.com/6space7/porter/internal/store"
)

type storeEnvVarService struct {
	queries *store.Queries
	box     *secretcrypto.SecretBox
}

func NewStoreEnvVarService(queries *store.Queries, box *secretcrypto.SecretBox) EnvVarService {
	return storeEnvVarService{queries: queries, box: box}
}

func (service storeEnvVarService) SetEnvVar(ctx context.Context, appID string, input SetEnvVarInput) (EnvVar, error) {
	value := input.Value
	if input.IsSecret {
		if service.box == nil {
			return EnvVar{}, fmt.Errorf("secret box is required for secret env vars")
		}
		encrypted, err := service.box.Encrypt(input.Value)
		if err != nil {
			return EnvVar{}, err
		}
		value = encrypted
	}

	row, err := service.queries.UpsertEnvVar(ctx, store.UpsertEnvVarParams{
		AppID:    appID,
		Key:      input.Key,
		Value:    value,
		IsSecret: boolInt(input.IsSecret),
	})
	if err != nil {
		return EnvVar{}, err
	}
	return service.envVarFromStore(row)
}

func (service storeEnvVarService) ListEnvVars(ctx context.Context, appID string) ([]EnvVar, error) {
	rows, err := service.queries.ListEnvVarsByApp(ctx, appID)
	if err != nil {
		return nil, err
	}

	envVars := make([]EnvVar, 0, len(rows))
	for _, row := range rows {
		envVar, err := service.envVarFromStore(row)
		if err != nil {
			return nil, err
		}
		envVars = append(envVars, envVar)
	}
	return envVars, nil
}

func (service storeEnvVarService) envVarFromStore(row store.EnvVar) (EnvVar, error) {
	value := row.Value
	isSecret := row.IsSecret == 1
	if isSecret {
		if service.box == nil {
			return EnvVar{}, fmt.Errorf("secret box is required for secret env vars")
		}
		decrypted, err := service.box.Decrypt(row.Value)
		if err != nil {
			return EnvVar{}, err
		}
		value = decrypted
	}

	return EnvVar{
		AppID:    row.AppID,
		Key:      row.Key,
		Value:    value,
		IsSecret: isSecret,
	}, nil
}

func boolInt(value bool) int64 {
	if value {
		return 1
	}
	return 0
}
