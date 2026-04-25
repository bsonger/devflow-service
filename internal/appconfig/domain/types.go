package domain

type EnvVar struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

type LabelItem struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

type File struct {
	Name    string `json:"name"`
	Content string `json:"content"`
}
