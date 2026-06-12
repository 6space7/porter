package install

import "path"

type Paths struct {
	ConfigDir     string
	DataDir       string
	DatabasePath  string
	MasterKeyPath string
	WorkspacePath string
}

func DefaultPaths() Paths {
	configDir := "/etc/porter"
	dataDir := "/var/lib/porter"
	return Paths{
		ConfigDir:     configDir,
		DataDir:       dataDir,
		DatabasePath:  path.Join(dataDir, "porter.db"),
		MasterKeyPath: path.Join(configDir, "master.key"),
		WorkspacePath: path.Join(dataDir, "work"),
	}
}
