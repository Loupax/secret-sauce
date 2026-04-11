package manifest

type Manifest struct {
	Env  map[string]string `toml:"env"`
	File map[string]string `toml:"file"`
}
