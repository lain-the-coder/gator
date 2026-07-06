package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"database/sql"

	"github.com/google/uuid"
	"github.com/lain-the-coder/gator/internal/config"
	"github.com/lain-the-coder/gator/internal/database"
	_ "github.com/lib/pq"
)

type state struct {
	cfg *config.Config
	db  *database.Queries
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
	_, err := s.db.GetUser(context.Background(), cmd.args[0])
	if err != nil {
		// If the name is missing, Postgres returns a sql.ErrNoRows error
		return fmt.Errorf("user does not exist: %w", err)
	}
	err = s.cfg.SetUser(cmd.args[0])
	if err != nil {
		return fmt.Errorf("error setting username: %w", err)
	}
	fmt.Println("Current username is set and user has logged in successfully!")
	return nil
}

func handlerReset(s *state, cmd command) error {
	err := s.db.ResetUsers(context.Background())
	if err != nil {
		return fmt.Errorf("error deletion did not complete successfully: %w", err)
	}
	fmt.Println("All rows have been deleted successfully and table has been reset!")
	return nil
}

func handlerRegister(s *state, cmd command) error {
	if len(cmd.args) == 0 {
		return fmt.Errorf("usage: %s <name>", cmd.name)
	}
	username := cmd.args[0]
	user, err := s.db.CreateUser(context.Background(), database.CreateUserParams{
		ID:        uuid.New(),
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
		Name:      username,
	})
	if err != nil {
		// A database error happened (most likely unique constraint violation)
		fmt.Printf("error creating user: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("User created successfully!")
	err = s.cfg.SetUser(user.Name)
	if err != nil {
		return fmt.Errorf("error setting username: %w", err)
	}
	fmt.Println("Current username is set and user has logged in successfully!")
	return nil
}

func handleGetUsers(s *state, cmd command) error {
	users, err := s.db.GetUsers(context.Background())
	if err != nil {
		return fmt.Errorf("error completing select all rows command: %w", err)
	}
	for _, user := range users {
		if user.Name == s.cfg.CurrentUsername {
			fmt.Printf("* %s (current)\n", user.Name)
		} else {
			fmt.Printf("* %s\n", user.Name)
		}
	}
	return nil
}

func main() {
	cfg, err := config.Read()
	if err != nil {
		log.Fatalf("error reading config file: %v", err)
	}
	rawDB, err := sql.Open("postgres", cfg.DbURL)
	if err != nil {
		log.Fatalf("error opening database: %v", err)
	}
	db := database.New(rawDB)
	s := state{
		cfg: &cfg,
		db:  db,
	}
	cmds := commands{
		registeredCommands: make(map[string]func(*state, command) error),
	}
	cmds.register("login", handlerLogin)
	cmds.register("register", handlerRegister)
	cmds.register("reset", handlerReset)
	cmds.register("users", handleGetUsers)
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
