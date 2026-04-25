package domain

var manifestRepo *Repo

func InitConfigRepo(c *Repo) {
	manifestRepo = c
}

func GetConfigRepo() *Repo {
	if manifestRepo == nil {
		panic("config repo not initialized")
	}
	return manifestRepo
}
