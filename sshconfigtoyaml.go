package sshw

import (
	"io"
	"os"

	"github.com/flosch/pongo2"
)

func tmplYaml() string {
	yamlSrc := `## default group
- name: default server group
  children:
{%- for host in sshConfigs %}
  - name: {{ host.Name }}
    user: {{ host.User }}
    host: {{ host.Host }}
    port: {{ host.Port }}
{% endfor %}
`
	tpl, err := pongo2.FromString(yamlSrc)
	if err != nil {
		l.Error(err)
	}

	yaml, err := tpl.Execute(pongo2.Context{
		"sshConfigs": GetConfig(),
	})
	if err != nil {
		l.Error(err)
	}
	return yaml
}

func checkFileIsExist(filename string) bool {
	if _, err := os.Stat(filename); os.IsNotExist(err) {
		return false
	}
	return true
}

func writeYaml(src string) {
	fileName := "sshconfig.yaml"
	var f *os.File
	var err error
	if checkFileIsExist(fileName) {
		f, err = os.OpenFile(fileName, os.O_APPEND, 0666)
		l.Error("file exist")
		os.Exit(1)
	} else {
		f, err = os.Create(fileName)
	}
	defer f.Close()
	_, err = io.WriteString(f, src)
	if err != nil {
	  l.Error(err)
	}
}

func GenYaml() {
	err := LoadSshConfig()
	if err != nil {
		l.Errorf("load ssh config error", err)
		os.Exit(1)
	}

	yaml := tmplYaml()
	writeYaml(yaml)
}
