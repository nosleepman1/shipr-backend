package tasks

import (
	"encoding/json"

	"github.com/hibiken/asynq"
)

const (
	TypeDeployApplication = "application:deploy"
)

type DeployApplicationPayload struct {
	DeploymentID  string `json:"deployment_id"`
	ApplicationID string `json:"application_id"`
	TriggerSource string `json:"trigger_source"`
}

func NewDeployApplicationTask(deploymentID, applicationID, triggerSource string) (*asynq.Task, error) {
	payload, err := json.Marshal(DeployApplicationPayload{
		DeploymentID:  deploymentID,
		ApplicationID: applicationID,
		TriggerSource: triggerSource,
	})
	if err != nil {
		return nil, err
	}
	return asynq.NewTask(TypeDeployApplication, payload, asynq.MaxRetry(3)), nil
}
