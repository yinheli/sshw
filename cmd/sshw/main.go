package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"os/user"
	"path"
	"runtime"
	"strings"
	"time"

	"github.com/manifoldco/promptui"
	"github.com/yinheli/sshw"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/terminal"
)

const prev = "-parent-"

var (
	Build = "devel"
	V     = flag.Bool("version", false, "show version")
	H     = flag.Bool("help", false, "show help")

	log = sshw.GetLogger()

	templates = &promptui.SelectTemplates{
		Label:    "✨ {{ . | green}}",
		Active:   "➤ {{ .Name | cyan }} {{if .Host}}{{if .User}}{{.User | faint}}{{`@` | faint}}{{end}}{{.Host | faint}}{{end}}",
		Inactive: "  {{.Name | faint}} {{if .Host}}{{if .User}}{{.User | faint}}{{`@` | faint}}{{end}}{{.Host | faint}}{{end}}",
	}
)

func main() {
	flag.Parse()
	if !flag.Parsed() {
		flag.Usage()
		return
	}

	if *H {
		flag.Usage()
		return
	}

	if *V {
		fmt.Println("sshw - ssh client wrapper for automatic login")
		fmt.Println("  git version:", Build)
		fmt.Println("  go version :", runtime.Version())
		return
	}

	err := sshw.LoadConfig()
	if err != nil {
		log.Error("load config error", err)
		os.Exit(1)
	}

	node := choose(nil, sshw.GetConfig())
	if node == nil {
		return
	}

	login(node)
}

func choose(parent, trees []*sshw.Node) *sshw.Node {
	prompt := promptui.Select{
		Label:     "select host",
		Items:     trees,
		Templates: templates,
		Size:      20,
		Searcher: func(input string, index int) bool {
			node := trees[index]
			content := fmt.Sprintf("%s %s %s", node.Name, node.User, node.Host)
			if strings.Contains(input, " ") {
				for _, key := range strings.Split(input, " ") {
					key = strings.TrimSpace(key)
					if key != "" {
						if !strings.Contains(content, key) {
							return false
						}
					}
				}
				return true
			} else {
				if strings.Contains(content, input) {
					return true
				}
			}
			return false
		},
	}
	index, _, err := prompt.Run()
	if err != nil {
		return nil
	}

	node := trees[index]
	if len(node.Children) > 0 {
		first := node.Children[0]
		if first.Name != prev {
			first = &sshw.Node{Name: prev}
			node.Children = append(node.Children[:0], append([]*sshw.Node{first}, node.Children...)...)
		}
		return choose(trees, node.Children)
	}

	if node.Name == prev {
		return choose(nil, parent)
	}

	return node
}

func login(node *sshw.Node) {
	u, err := user.Current()
	if err != nil {
		log.Error(err)
		return
	}

	var authMethods []ssh.AuthMethod

	var b []byte
	if node.KeyPath == "" {
		b, err = ioutil.ReadFile(path.Join(u.HomeDir, ".ssh/id_rsa"))
	} else {
		b, err = ioutil.ReadFile(node.KeyPath)
	}
	if err == nil {
		signer, err := ssh.ParsePrivateKey(b)
		if err == nil {
			authMethods = append(authMethods, ssh.PublicKeys(signer))
		}
	}

	if node.Password != "" {
		interactive := func(user, instruction string, questions []string, echos []bool) (answers []string, err error) {
			answers = make([]string, len(questions))
			for n := range questions {
				answers[n] = node.Password
			}

			return answers, nil
		}
		authMethods = append(authMethods, ssh.KeyboardInteractive(interactive))
		authMethods = append(authMethods, ssh.Password(node.Password))
	}

	username := node.User
	host := node.Host
	port := node.Port

	if username == "" {
		username = "root"
	}
	if port <= 0 {
		port = 22
	}

	config := &ssh.ClientConfig{
		User:            username,
		Auth:            authMethods,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         time.Second * 10,
	}

	config.SetDefaults()
	config.Ciphers = append(config.Ciphers, "aes128-cbc", "3des-cbc", "blowfish-cbc", "cast128-cbc", "aes192-cbc", "aes256-cbc")

	client, err := ssh.Dial("tcp", fmt.Sprintf("%s:%d", host, port), config)
	if err != nil {
		log.Error(err)
		return
	}
	defer client.Close()

	log.Infof("connect server ssh -p %d %s@%s password:%s  version: %s\n", port, username, host, node.Password, string(client.ServerVersion()))

	session, err := client.NewSession()
	if err != nil {
		log.Error(err)
		return
	}
	defer session.Close()

	fd := int(os.Stdin.Fd())
	state, err := terminal.MakeRaw(fd)
	if err != nil {
		log.Error(err)
		return
	}
	defer terminal.Restore(fd, state)

	w, h, err := terminal.GetSize(fd)
	if err != nil {
		log.Error(err)
		return
	}

	modes := ssh.TerminalModes{
		ssh.ECHO:          1,
		ssh.TTY_OP_ISPEED: 14400,
		ssh.TTY_OP_OSPEED: 14400,
	}
	err = session.RequestPty("xterm", h, w, modes)
	if err != nil {
		log.Error(err)
		return
	}

	session.Stdout = os.Stdout
	session.Stderr = os.Stderr
	session.Stdin = os.Stdin

	err = session.Shell()
	if err != nil {
		log.Error(err)
		return
	}

	session.Wait()
}
