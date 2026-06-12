package services

type DeployRequest struct {
	ServiceID     string
	TemplateSlug  string
	Image         string
	Command       []string
	ContainerName string
	InternalPort  int64
	Env           map[string]string
	Volumes       []VolumeSpec
}
