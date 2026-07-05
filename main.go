package main

import (
	"fmt"
	"log"
	"os"

	"github.com/lain-the-coder/gator/internal/config"
)

type state struct {
	cfg *config.Config
}

type command struct {
	name string
	args []string
}

type commands struct {
	registeredCommands map[string]func(*state, command) error
}

func (c *commands) register(name string, f func(*state, command) error) {
	c.registeredCommands[name] = f
}

func (c *commands) run(s *state, cmd command) error {
	f, exists := c.registeredCommands[cmd.name]
	if !exists {
		return fmt.Errorf("Command is not registered")
	}
	err := f(s, cmd)
	if err != nil {
		return fmt.Errorf("error executing command: %w", err)
	}
	return nil
}

func handlerLogin(s *state, cmd command) error {
	if len(cmd.args) == 0 {
		return fmt.Errorf("error - username is required to login")
	}
	err := s.cfg.SetUser(cmd.args[0])
	if err != nil {
		return fmt.Errorf("error setting username: %w", err)
	}
	fmt.Println("Current username is set and user has logged in successfully!")
	return nil
}

func main() {
	cfg, err := config.Read()
	if err != nil {
		log.Fatalf("error reading config file: %v", err)
	}
	s := state{
		cfg: &cfg,
	}
	cmds := commands{
		registeredCommands: make(map[string]func(*state, command) error),
	}
	cmds.register("login", handlerLogin)
	if len(os.Args) < 2 {
		log.Fatalln("error - too few arguments")
	}
	cmdName := os.Args[1]
	cmdArgs := os.Args[2:]
	cmd := command{
		name: cmdName,
		args: cmdArgs,
	}
	err = cmds.run(&s, cmd)
	if err != nil {
		log.Fatalf("error running command: %v", err)
	}
}
