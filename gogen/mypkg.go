package gogen

//go:generate go run -tags generate ./gen/main.go mypkg.gen

func MyCode() string {
	return "important things are happening"
}
