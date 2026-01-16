package sshw

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"os"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/atrox/homedir"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
	"golang.org/x/crypto/ssh/terminal"
)

var (
	DefaultCiphers = []string{
		"aes128-ctr",
		"aes192-ctr",
		"aes256-ctr",
		"aes128-gcm@openssh.com",
		"chacha20-poly1305@openssh.com",
		"arcfour256",
		"arcfour128",
		"arcfour",
		"aes128-cbc",
		"3des-cbc",
		"blowfish-cbc",
		"cast128-cbc",
		"aes192-cbc",
		"aes256-cbc",
	}
)

type Client interface {
	Login()
}

type cleanupFunc func()

type defaultClient struct {
	clientConfig *ssh.ClientConfig
	node         *Node
	cleanups     []cleanupFunc
}

func getAgentConn(agentPath string) (net.Conn, error) {
	if len(agentPath) == 0 {
		return nil, nil
	}
	expandedPath, err := homedir.Expand(agentPath)
	if err != nil {
		return nil, err
	}
	// IdentityAgent
	//  Specifies the UNIX-domain socket used to communicate with the
	//  authentication agent.
	return net.DialTimeout("unix", expandedPath, time.Second*10)
}

func setupAgentAuth(node *Node) (ssh.AuthMethod, cleanupFunc, error) {
	if node.AgentPath == "" {
		return nil, nil, nil
	}

	conn, err := getAgentConn(node.AgentPath)
	if err != nil {
		return nil, nil, err
	}
	client := agent.NewClient(conn)
	return ssh.PublicKeysCallback(client.Signers), func() { conn.Close() }, nil
}

func setupKeyFileAuth(node *Node) (ssh.AuthMethod, cleanupFunc, error) {
	keyPath := node.KeyPath
	if keyPath == "" {
		return nil, nil, nil
	}

	keyPath, err := homedir.Expand(keyPath)
	if err != nil {
		return nil, nil, err
	}

	bytes, err := os.ReadFile(keyPath)
	if err != nil {
		return nil, nil, err
	}

	var signer ssh.Signer
	if node.Passphrase != "" {
		signer, err = ssh.ParsePrivateKeyWithPassphrase(bytes, []byte(node.Passphrase))
	} else {
		signer, err = ssh.ParsePrivateKey(bytes)
	}
	if err != nil {
		return nil, nil, err
	}
	return ssh.PublicKeys(signer), nil, nil
}

func setupDefaultKeyAuth(node *Node) (ssh.AuthMethod, cleanupFunc, error) {
	u, err := user.Current()
	if err != nil {
		return nil, nil, err
	}
	keyPath := filepath.Join(u.HomeDir, ".ssh/id_rsa")
	bytes, err := os.ReadFile(keyPath)
	if err == nil && len(bytes) > 0 {
		var signer ssh.Signer
		if node.Passphrase != "" {
			signer, err = ssh.ParsePrivateKeyWithPassphrase(bytes, []byte(node.Passphrase))
		} else {
			signer, err = ssh.ParsePrivateKey(bytes)
		}
		if err == nil {
			return ssh.PublicKeys(signer), nil, nil
		}
	}
	return nil, nil, nil
}

func setupPasswordAuth(node *Node) (ssh.AuthMethod, cleanupFunc, error) {
	if node.Password == "" {
		return nil, nil, nil
	}
	return ssh.Password(node.Password), nil, nil
}

func setupKeyboardAuth(node *Node) (ssh.AuthMethod, cleanupFunc, error) {
	return ssh.KeyboardInteractive(func(user, instruction string, questions []string, echos []bool) ([]string, error) {
		answers := make([]string, 0, len(questions))
		for i, q := range questions {
			fmt.Print(q)
			if echos[i] {
				scan := bufio.NewScanner(os.Stdin)
				if scan.Scan() {
					answers = append(answers, scan.Text())
				}
				err := scan.Err()
				if err != nil {
					return nil, err
				}
			} else {
				b, err := terminal.ReadPassword(int(syscall.Stdin))
				if err != nil {
					return nil, err
				}
				fmt.Println()
				answers = append(answers, string(b))
			}
		}
		return answers, nil
	}), nil, nil
}

func genSSHConfig(node *Node) *defaultClient {
	// support multiple auth methods
	// order: agent > key file > default key file > password > keyboard interactive
	fetchers := []func(node *Node) (ssh.AuthMethod, cleanupFunc, error){
		setupAgentAuth,
		setupKeyFileAuth,
		setupDefaultKeyAuth,
		setupPasswordAuth,
		setupKeyboardAuth,
	}

	var authMethods []ssh.AuthMethod
	var cleanups []cleanupFunc
	for _, fetcher := range fetchers {
		authMethod, cleanup, err := fetcher(node)
		if err == nil && authMethod != nil {
			authMethods = append(authMethods, authMethod)
		}
		if cleanup != nil {
			cleanups = append(cleanups, cleanup)
		}
		if err != nil {
			l.Error(err)
		}
	}

	config := &ssh.ClientConfig{
		User:            node.user(),
		Auth:            authMethods,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         time.Second * 10,
	}

	config.SetDefaults()
	config.Ciphers = append(config.Ciphers, DefaultCiphers...)

	return &defaultClient{
		clientConfig: config,
		node:         node,
		cleanups:     cleanups,
	}
}

func NewClient(node *Node) Client {
	return genSSHConfig(node)
}

func (c *defaultClient) close() {
	for _, cleanup := range c.cleanups {
		cleanup()
	}
}

func (c *defaultClient) Login() {
	defer c.close()

	host := c.node.Host
	port := strconv.Itoa(c.node.port())
	jNodes := c.node.Jump

	var client *ssh.Client

	if len(jNodes) > 0 {
		jNode := jNodes[0]
		jc := genSSHConfig(jNode)
		proxyClient, err := ssh.Dial("tcp", net.JoinHostPort(jNode.Host, strconv.Itoa(jNode.port())), jc.clientConfig)
		if err != nil {
			l.Error(err)
			return
		}
		conn, err := proxyClient.Dial("tcp", net.JoinHostPort(host, port))
		if err != nil {
			l.Error(err)
			return
		}
		ncc, chans, reqs, err := ssh.NewClientConn(conn, net.JoinHostPort(host, port), c.clientConfig)
		if err != nil {
			l.Error(err)
			return
		}
		client = ssh.NewClient(ncc, chans, reqs)
	} else {
		client1, err := ssh.Dial("tcp", net.JoinHostPort(host, port), c.clientConfig)
		client = client1
		if err != nil {
			msg := err.Error()
			// use terminal password retry
			if strings.Contains(msg, "no supported methods remain") && !strings.Contains(msg, "password") {
				fmt.Printf("%s@%s's password:", c.clientConfig.User, host)
				var b []byte
				b, err = terminal.ReadPassword(int(syscall.Stdin))
				if err == nil {
					p := string(b)
					if p != "" {
						c.clientConfig.Auth = append(c.clientConfig.Auth, ssh.Password(p))
					}
					fmt.Println()
					client, err = ssh.Dial("tcp", net.JoinHostPort(host, port), c.clientConfig)
				}
			}
		}
		if err != nil {
			l.Error(err)
			return
		}
	}
	defer client.Close()

	l.Infof("connect server ssh -p %d %s@%s version: %s\n", c.node.port(), c.node.user(), host, string(client.ServerVersion()))

	session, err := client.NewSession()
	if err != nil {
		l.Error(err)
		return
	}
	defer session.Close()

	fd := int(os.Stdin.Fd())
	state, err := terminal.MakeRaw(fd)
	if err != nil {
		l.Error(err)
		return
	}
	defer terminal.Restore(fd, state)

	//changed fd to int(os.Stdout.Fd()) because terminal.GetSize(fd) doesn't work in Windows
	//reference: https://github.com/golang/go/issues/20388
	w, h, err := terminal.GetSize(int(os.Stdout.Fd()))

	if err != nil {
		l.Error(err)
		return
	}

	modes := ssh.TerminalModes{
		ssh.ECHO:          1,
		ssh.TTY_OP_ISPEED: 14400,
		ssh.TTY_OP_OSPEED: 14400,
	}
	err = session.RequestPty("xterm", h, w, modes)
	if err != nil {
		l.Error(err)
		return
	}

	session.Stdout = os.Stdout
	session.Stderr = os.Stderr
	stdinPipe, err := session.StdinPipe()
	if err != nil {
		l.Error(err)
		return
	}

	err = session.Shell()
	if err != nil {
		l.Error(err)
		return
	}

	// then callback
	for i := range c.node.CallbackShells {
		shell := c.node.CallbackShells[i]
		time.Sleep(shell.Delay * time.Millisecond)
		stdinPipe.Write([]byte(shell.Cmd + "\r"))
	}

	// change stdin to user
	go func() {
		_, err = io.Copy(stdinPipe, os.Stdin)
		l.Error(err)
		session.Close()
	}()

	// interval get terminal size
	// fix resize issue
	go func() {
		var (
			ow = w
			oh = h
		)
		for {
			cw, ch, err := terminal.GetSize(fd)
			if err != nil {
				break
			}

			if cw != ow || ch != oh {
				err = session.WindowChange(ch, cw)
				if err != nil {
					break
				}
				ow = cw
				oh = ch
			}
			time.Sleep(time.Second)
		}
	}()

	// send keepalive
	go func() {
		for {
			time.Sleep(time.Second * 10)
			client.SendRequest("keepalive@openssh.com", false, nil)
		}
	}()

	session.Wait()
}
