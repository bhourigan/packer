package consul

import (
	"fmt"
	"strings"

        "github.com/hashicorp/consul/api"
	"github.com/mitchellh/mapstructure"
	"github.com/mitchellh/packer/common"
	"github.com/mitchellh/packer/packer"
)

var builtins = map[string]string{
	"mitchellh.amazonebs": "amazonebs",
	"mitchellh.amazon.instance": "amazoninstance",
}

// Artifacts can return a string for this state key and the post-processor
// will use automatically use this as the type. The user's value overrides
// this if `artifact_type_override` is set to true.
const ArtifactStateType = "consul.artifact.type"

// Artifacts can return a map[string]string for this state key and this
// post-processor will automatically merge it into the metadata for any
// uploaded artifact versions.
const ArtifactStateMetadata = "consul.artifact.metadata"

type Config struct {
	common.PackerConfig `mapstructure:",squash"`

	Artifact     string
	Type         string `mapstructure:"artifact_type"`
	TypeOverride bool   `mapstructure:"artifact_type_override"`
	Metadata     map[string]string

	Address      string `mapstructure:"address"`
	Scheme       string `mapstructure:"scheme"`
	Datacenter   string `mapstructure:"datacenter"`
	Token        string `mapstructure:"token"`

	tpl *packer.ConfigTemplate
}

type PostProcessor struct {
	config Config
	client *api.Client
}

func (p *PostProcessor) Configure(raws ...interface{}) error {
	_, err := common.DecodeConfig(&p.config, raws...)
	if err != nil {
		return err
	}

	p.config.tpl, err = packer.NewConfigTemplate()
	if err != nil {
		return err
	}
	p.config.tpl.UserVars = p.config.PackerUserVars

	templates := map[string]*string{
		"address":    &p.config.Address,
		"scheme":     &p.config.Scheme,
		"datacenter": &p.config.Datacenter,
		"token":      &p.config.Token,
	}

	errs := new(packer.MultiError)
	for key, ptr := range templates {
		*ptr, err = p.config.tpl.Process(*ptr, nil)
		if err != nil {
			errs = packer.MultiErrorAppend(
				errs, fmt.Errorf("Error processing %s: %s", key, err))
		}
	}

	required := map[string]*string{
		"address":      &p.config.Address,
	}

	for key, ptr := range required {
		if *ptr == "" {
			errs = packer.MultiErrorAppend(
				errs, fmt.Errorf("%s must be set", key))
		}
	}

	if len(errs.Errors) > 0 {
		return errs
	}

	config := api.DefaultConfig()
	if p.config.Address != "" {
		config.Address = p.config.Address
	}

	if p.config.Scheme != "" {
		config.Scheme = p.config.Scheme
	}

	if p.config.Datacenter != "" {
		config.Datacenter = p.config.Datacenter
	}

	if p.config.Token != "" {
		config.Token = p.config.Token
	}

        p.client, err = api.NewClient(config)
        if err != nil {
                return err
        }

	return nil
}

func (p *PostProcessor) PostProcess(ui packer.Ui, artifact packer.Artifact) (packer.Artifact, bool, error) {
	_, ok := builtins[artifact.BuilderId()]
	if !ok {
		return nil, false, fmt.Errorf(
			"Unsupported artifact type: %s", artifact.BuilderId())
	}

/*	kv := p.client.KV()
	artifacts := p.metadata(artifact)
*/
	for _, regions := range strings.Split(artifact.Id(), ",") {
		parts := strings.Split(regions, ":")
		if len(parts) != 2 {
			err := fmt.Errorf("Poorly formatted artifact ID: %s", artifact.Id())
			return nil, false, err
		}

		ui.Message(fmt.Sprintf("AMI ID part 0: %s", parts[0]))
		ui.Message(fmt.Sprintf("AMI ID part 1: %s", parts[1]))
	}

        Id := artifact.Id()
        ui.Message(fmt.Sprintf("AMI IDs: %s", Id))

/*
	data := &api.KVPair{Key: artifact.Id(), Value: p.metadata(artifact)}

	_, err := kv.Put(data, nil)
	if err != nil {
    		return nil, false, err
	}
*/
	return artifact, false, nil
}

func (p *PostProcessor) metadata(artifact packer.Artifact) map[string]string {
	var metadata map[string]string
	metadataRaw := artifact.State(ArtifactStateMetadata)
	if metadataRaw != nil {
		if err := mapstructure.Decode(metadataRaw, &metadata); err != nil {
			panic(err)
		}
	}

	if p.config.Metadata != nil {
		// If we have no extra metadata, just return as-is
		if metadata == nil {
			return p.config.Metadata
		}

		// Merge the metadata
		for k, v := range p.config.Metadata {
			metadata[k] = v
		}
	}

	return metadata
}

func (p *PostProcessor) artifactType(artifact packer.Artifact) string {
	if !p.config.TypeOverride {
		if v := artifact.State(ArtifactStateType); v != nil {
			return v.(string)
		}
	}

	return p.config.Type
}
