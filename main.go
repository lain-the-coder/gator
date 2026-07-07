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

func middlewareLoggedIn(handler func(s *state, cmd command, user database.User) error) func(*state, command) error {
	return func(s *state, cmd command) error {
		user, err := s.db.GetUser(context.Background(), s.cfg.CurrentUsername)
		if err != nil {
			return fmt.Errorf("user does not exist: %w", err)
		}
		return handler(s, cmd, user)
	}
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

func handlerGetUsers(s *state, cmd command) error {
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

func handlerAgg(s *state, cmd command) error {
	if len(cmd.args) != 0 {
		return fmt.Errorf("this command takes no arguments")
	}
	rssfeed, err := fetchFeed(context.Background(), "https://www.wagslane.dev/index.xml")
	if err != nil {
		return fmt.Errorf("error completing successful fetch feed request: %w", err)
	}
	fmt.Printf("%+v\n", rssfeed)
	return nil
}

func handlerAddFeed(s *state, cmd command, user database.User) error {
	if len(cmd.args) != 2 {
		return fmt.Errorf("this command needs two arguments and its usage is as follows: gator addfeed <name> <url>")
	}
	feed, err := s.db.CreateFeed(context.Background(), database.CreateFeedParams{
		ID:        uuid.New(),
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
		Name:      cmd.args[0],
		Url:       cmd.args[1],
		UserID:    user.ID,
	})
	if err != nil {
		return fmt.Errorf("error creating feed: %w", err)
	}
	fmt.Println("Feed created successfully!")
	feedFollow, err := s.db.CreateFeedFollow(context.Background(), database.CreateFeedFollowParams{
		ID:        uuid.New(),
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
		UserID:    user.ID,
		FeedID:    feed.ID,
	})
	if err != nil {
		return fmt.Errorf("error creating feed follow record: %w", err)
	}
	fmt.Println("Successfully created feed follow record!")
	fmt.Printf("Feed Name: %s; Feed URL: %s; Feed Owner: %s\n", feed.Name, feed.Url, user.Name)
	fmt.Printf("User %s is following Feed %s\n", feedFollow.UserName, feedFollow.FeedName)
	return nil
}

func handlerListFeeds(s *state, cmd command) error {
	if len(cmd.args) != 0 {
		return fmt.Errorf("this command does not need any arguments and it's usage is as follows: gator feeds")
	}
	feeds, err := s.db.GetFeeds(context.Background())
	if err != nil {
		return fmt.Errorf("error fetching all feeds available within db: %w", err)
	}
	fmt.Printf("List of %d feeds fetched successfully!\n", len(feeds))
	for _, feed := range feeds {
		fmt.Printf("Feed name: %s; Feed URL: %s; Username: %s\n", feed.FeedName, feed.Url, feed.UserName)
	}
	return nil
}

func handlerFollowFeeds(s *state, cmd command, user database.User) error {
	if len(cmd.args) != 1 {
		return fmt.Errorf("this command only needs one argument and it's usage is as follows: gator follow <url>")
	}
	feed, err := s.db.GetFeedByURL(context.Background(), cmd.args[0])
	if err != nil {
		return fmt.Errorf("feed does not exist: %w", err)
	}
	feedFollow, err := s.db.CreateFeedFollow(context.Background(), database.CreateFeedFollowParams{
		ID:        uuid.New(),
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
		UserID:    user.ID,
		FeedID:    feed.ID,
	})
	if err != nil {
		return fmt.Errorf("error creating feed follow record: %w", err)
	}
	fmt.Println("Successfully created feed follow record!")
	fmt.Printf("User %s is following Feed %s\n", feedFollow.UserName, feedFollow.FeedName)
	return nil
}

func handlerFollowedFeeds(s *state, cmd command, user database.User) error {
	if len(cmd.args) != 0 {
		return fmt.Errorf("this command does not need any arguments and it's usage is as follows: gator following")
	}
	feedsFollowing, err := s.db.GetFeedFollowsForUser(context.Background(), user.ID)
	if err != nil {
		return fmt.Errorf("error fetching all feeds followed by user: %w", err)
	}
	fmt.Println("Successfully fetched all rows of feeds followed by user")
	for i, feedFollowing := range feedsFollowing {
		fmt.Printf("Feed %d: %s\n", i, feedFollowing.FeedName)
	}
	return nil
}

func handlerUnfollow(s *state, cmd command, user database.User) error {
	if len(cmd.args) != 1 {
		return fmt.Errorf("usage: gator unfollow <feed_url>")
	}
	feed, err := s.db.GetFeedByURL(context.Background(), cmd.args[0])
	if err != nil {
		return fmt.Errorf("could not find a feed matching url: %w", err)
	}
	err = s.db.DeleteFeedFollow(context.Background(), database.DeleteFeedFollowParams{
		UserID: user.ID,
		FeedID: feed.ID,
	})
	if err != nil {
		return fmt.Errorf("failed to remove feed follow record: %w", err)
	}
	fmt.Printf("Sucessfully unfollowed feed: %s\n", feed.Name)
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
	cmds.register("users", handlerGetUsers)
	cmds.register("agg", handlerAgg)
	cmds.register("addfeed", middlewareLoggedIn(handlerAddFeed))
	cmds.register("feeds", handlerListFeeds)
	cmds.register("follow", middlewareLoggedIn(handlerFollowFeeds))
	cmds.register("following", middlewareLoggedIn(handlerFollowedFeeds))
	cmds.register("unfollow", middlewareLoggedIn(handlerUnfollow))
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
