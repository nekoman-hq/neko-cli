package release

/*
@Author     Benjamin Senekowitsch
@Contact    senekowitsch@nekoman.at
@Since      18.12.2025
*/

import "fmt"

var tools = make(map[string]Tool)

func Register(t Tool) {
	tools[t.Name()] = t
}

func Get(name string) (Tool, error) {
	if t, ok := tools[name]; ok {
		return t, nil
	}
	return nil, fmt.Errorf("unknown release system: %s", name)
}
