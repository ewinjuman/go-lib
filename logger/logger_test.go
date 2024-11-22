package logger

type object struct {
	Pin      string     `json:"pin"`
	Password string     `json:"password"`
	Token    string     `json:"token"`
	Name     string     `json:"name"`
	JsonData objectdata `json:"jsonData"`
}

type objectdata struct {
	Email string `json:"email"`
}
