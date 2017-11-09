package commands

import (
	"io/ioutil"

	"github.com/cloudfoundry/bosh-bootloader/storage"
)

type AWSCreateLBs struct {
	cloudConfigManager   cloudConfigManager
	stateStore           stateStore
	stateValidator       stateValidator
	terraformManager     terraformManager
	environmentValidator EnvironmentValidator
}

type AWSCreateLBsConfig struct {
	LBType    string
	CertPath  string
	KeyPath   string
	ChainPath string
	Domain    string
}

type EnvironmentValidator interface {
	Validate(state storage.State) error
}

func NewAWSCreateLBs(cloudConfigManager cloudConfigManager, stateStore stateStore,
	terraformManager terraformManager, environmentValidator EnvironmentValidator) AWSCreateLBs {
	return AWSCreateLBs{
		cloudConfigManager:   cloudConfigManager,
		stateStore:           stateStore,
		terraformManager:     terraformManager,
		environmentValidator: environmentValidator,
	}
}

func (c AWSCreateLBs) Execute(config CreateLBsConfig, state storage.State) error {
	if state.LB.Type != "" {
		if config.Domain == "" {
			config.Domain = state.LB.Domain
		}

		if config.LBType == "" {
			config.LBType = state.LB.Type
		}
	}

	if err := c.environmentValidator.Validate(state); err != nil {
		return err
	}

	certContents, err := ioutil.ReadFile(config.CertPath)
	if err != nil {
		return err
	}

	keyContents, err := ioutil.ReadFile(config.KeyPath)
	if err != nil {
		return err
	}

	state.LB.Cert = string(certContents)
	state.LB.Key = string(keyContents)

	if config.ChainPath != "" {
		chainContents, err := ioutil.ReadFile(config.ChainPath)
		if err != nil {
			return err
		}

		state.LB.Chain = string(chainContents)
	}

	if config.Domain != "" {
		state.LB.Domain = config.Domain
	}

	state.LB.Type = config.LBType

	if err := c.stateStore.Set(state); err != nil {
		return err
	}

	if err := c.terraformManager.Init(state); err != nil {
		return err
	}

	state, err = c.terraformManager.Apply(state)
	if err != nil {
		return handleTerraformError(err, state, c.stateStore)
	}

	if err := c.stateStore.Set(state); err != nil {
		return err
	}

	if !state.NoDirector {
		err = c.cloudConfigManager.Initialize(state)
		if err != nil {
			return err
		}
		err = c.cloudConfigManager.Update(state)
		if err != nil {
			return err
		}
	}

	return nil
}
