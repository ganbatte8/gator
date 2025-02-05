package main

import "fmt"
import "github.com/ganbatte8/gator/internal/config"
import "github.com/ganbatte8/gator/internal/database"
import "os"
import _ "github.com/lib/pq"
import "time"
import "github.com/google/uuid"
import "context"
import "database/sql"
import "net/http"
import "io"
import "encoding/xml"
import "html"
import "bytes"
import "errors"
import "strconv"
import "strings"

type state struct {
  config *config.Config
  db *database.Queries
}

type command struct {
  name string
  args []string
}

type commands struct {
  m map[string]func(*state, command) error

}

func handlerLogin(s *state, cmd command) error {
  if len(cmd.args) == 0 {
    return fmt.Errorf("no arguments given for login command, expected 1\n")
  }
  username := cmd.args[0]
  ctx := context.Background()
  _, err := s.db.GetUser(ctx, username)
  if err != nil {
    fmt.Printf("GetUser error: %s\n", err)  // maybe username doesn't exist
    os.Exit(1)
  }

  err = s.config.SetUser(username)
  if err != nil {
    return fmt.Errorf("SetUser() errored: %s\n", err)
  }
  fmt.Println("User has been set")

  return err
}

func handlerRegister(s *state, cmd command) error {
  if len(cmd.args) == 0 {
    return fmt.Errorf("no arguments given for register command, expected 1\n")
  }
  username := cmd.args[0]
  
  now := time.Now()
  params := database.CreateUserParams {
    ID: uuid.New(),
    CreatedAt: now,
    UpdatedAt: now,
    Name: username,
  }

  ctx := context.Background()  // type Context; empty context
  user, err := s.db.CreateUser(ctx, params)
  if err != nil {
    // presumably run when user already exists?
    fmt.Printf("CreateUser errored: %s\n", err)
    os.Exit(1)
    return err
  }

  fmt.Printf("Added user %s id:%v time:%v", user.Name, user.ID, user.CreatedAt)
  
  err = s.config.SetUser(username)
  if err != nil {
    return fmt.Errorf("SetUser() errored: %s\n", err)
  }
  fmt.Println("User has been set")

  return nil
}

func handlerDeleteAllUsers(s *state, cmd command) error {
  ctx := context.Background();
  err := s.db.DeleteAllUsers(ctx)
  if err != nil {
    fmt.Printf("DeleteAllUsers error: %s\n", err)
  } else {
    fmt.Println("Deleted all users")
  }
  return err
}

func handlerUsers(s *state, cmd command) error {
  ctx := context.Background()
  users, err := s.db.GetAllUsers(ctx)
  if err != nil {
    fmt.Printf("GetAllUsers error: %s\n", err)
  }

  for _, u := range users {
    isCurrent := u.Name == s.config.CurrentUsername
    if isCurrent {
      fmt.Printf("* %s (current)\n", u.Name)
    } else {  
      fmt.Printf("* %s\n", u.Name)
    }
  }

  return err
}

func (c *commands) register(name string, f func(*state, command) error) {
  c.m[name] = f
}

func (c *commands) run(s *state, cmd command) error {
  f, ok := c.m[cmd.name]
  if !ok {
    return fmt.Errorf("Tried to run command %s but it doesn't exist\n", cmd.name)
  }
  err := f(s, cmd)
  if err != nil {
    return fmt.Errorf("Command %s errored: %s", cmd.name, err)
  }
  return nil
}

func fetchFeed(ctx context.Context, feedURL string) (*RSSFeed, error) {
  request, err := http.NewRequestWithContext(ctx, "GET", feedURL, nil)
  if err != nil {
    fmt.Printf("Error making new request: %s", err)
    return nil, err
  }
  
  request.Header.Set("User-Agent", "gator")

  client := http.Client{}

  response, err := client.Do(request)

  if err != nil {
    fmt.Printf("Error doing request or receiving response: %s\n", err)
    return nil, err
  }

  theBytes, err := io.ReadAll(response.Body)
  if err != nil {
    fmt.Printf("Error reading response: %s\n", err)
    return nil, err
  }

  //fmt.Printf("Received body: %s\n", string(theBytes))

  var feed RSSFeed

  reader := bytes.NewReader(theBytes)
  decoder := xml.NewDecoder(reader)
  decoder.Strict = false
  err = decoder.Decode(&feed)
  //err = xml.Unmarshal(bytes, &feed)
  
  if err != nil {
    fmt.Printf("Error parsing xml: %s\n", err)
    return nil, err
  }

  feed.Channel.Title = html.UnescapeString(feed.Channel.Title)
  feed.Channel.Description = html.UnescapeString(feed.Channel.Description)
  for i := 0; i < len(feed.Channel.Item); i++ {
    item := &feed.Channel.Item[i]
    item.Title = html.UnescapeString(item.Title)
    item.Description = html.UnescapeString(item.Description)
  }

  return &feed, err
}

func handlerAgg(s *state, cmd command) error {
  if len(cmd.args) < 1 {
    return errors.New("agg command requires one argument")
  }
  time_between_reqs, err := time.ParseDuration(cmd.args[0])
  if err != nil {
    return errors.New("error parsing duration argument")
  }

  fmt.Printf("Collecting feeds every %v", time_between_reqs)

  ticker := time.NewTicker(time_between_reqs)
  for ; ; <-ticker.C {
    scrapeFeeds(s)
  }
  // NOTE: do not DOS the servers you're fetching from.


/*
  url := "https://www.wagslane.dev/index.xml"
  feed, err := fetchFeed(context.Background(), url)
  
  if err != nil {
    fmt.Printf("fetchFeed errored\n")
    return err
  }

  n := len(feed.Channel.Item)
  fmt.Printf("Title:%s\nLink:%s\nDescription:%s\nItemCount:%d\n",
             feed.Channel.Title, feed.Channel.Link, feed.Channel.Description, n)

  for i := 0; i < n; i++ {
    item := feed.Channel.Item[i]
    fmt.Printf("Item %d:\nTitle:%s\nLink:%s\nDescription:%s\nPubDate:%s\n",
               i, item.Title, item.Link, item.Description, item.PubDate)
  }
*/
  return err
}

func middlewareLoggedIn(handler func(s *state, cmd command, user database.User) error) func(*state, command) error {

  return func(s *state, cmd command) error {  
    userName := s.config.CurrentUsername
    dbUser, err := s.db.GetUser(context.Background(), userName)
    if err != nil {
      fmt.Printf("GetUser errored:%s\n", err)
      return err
    }
    err = handler(s, cmd, dbUser)
    return err
  }
}


func handlerAddFeed(s *state, cmd command, dbUser database.User) error {
  if len(cmd.args) < 2 {
    errString := fmt.Sprintf("addFeed wants 2 arguments, got %d", len(cmd.args))
    return errors.New(errString)
  }
  feedName := cmd.args[0]
  url := cmd.args[1]

  ctx := context.Background()

  feedParams := database.CreateFeedParams {
    ID: uuid.New(),
    Name: feedName,
    Url: url,
    UserID: dbUser.ID,
  }

  feed, err := s.db.CreateFeed(ctx, feedParams)
  if err != nil {
    fmt.Printf("CreateFeed errored:%s\n", err)
    return err
  }
  
  fmt.Printf("feed ID:%s Name:%s Url:%s UserID:%s\n",
             feed.ID, feed.Name, feed.Url, feed.UserID)

  cffParams := database.CreateFeedFollowParams {
    ID: uuid.New(),
    UserID: dbUser.ID,
    FeedID: feed.ID,
  }
  _, err = s.db.CreateFeedFollow(ctx, cffParams)
  if err != nil {
     fmt.Printf("CreateFeedFollow error:%s\n", err)
     return err
  }

  return nil
}

func handlerFeeds(s *state, cmd command) error {
  ctx := context.Background()
  feeds, err := s.db.GetAllFeeds(ctx)
  if err != nil {
    fmt.Printf("GetAllFeeds errored:%s\n", err)
    return err
  }

  // Ideally, we should probably do one query that joins feeds and users
  // instead of querying users multiple times in a loop like this

  for i := 0; i < len(feeds); i++ {
    feed := feeds[i]
    dbUser, err := s.db.GetUserFromID(ctx, feed.UserID)
    if err != nil {
      fmt.Printf("GetUserFromID error: %s\n", err)
      return err
    }

    fmt.Printf("Name:%s Url:%s username:%s\n", feed.Name, feed.Url, dbUser.Name)
  }
  return nil
}

func handlerFollow(s *state, cmd command, dbUser database.User) error {
  if len(cmd.args) < 1 {
    return errors.New("handlerFollow wants one argument")
  }

  ctx := context.Background()
  url := cmd.args[0]

  dbFeed, err := s.db.GetFeedFromUrl(ctx, url)
  if err != nil {
    fmt.Printf("GetFeedFromUrl errored:%s\n", err)
    return err
  }

  cffParams := database.CreateFeedFollowParams {
    ID: uuid.New(),
    UserID: dbUser.ID,
    FeedID: dbFeed.ID,
  }
  _, err = s.db.CreateFeedFollow(ctx, cffParams)
  if err != nil {
    fmt.Printf("CreateFeedFollow errored:%s\n", err)
    return err
  }
  fmt.Printf("feed name:%s current user:%s\n", dbFeed.Name, dbUser.Name)
  return nil
}

func handlerFollowing(s *state, cmd command, dbUser database.User) error {
  ctx := context.Background()

  dbFFs, err := s.db.GetFeedFollowsForUser(ctx, dbUser.ID)
  if err != nil {
    fmt.Printf("GetFeedFollowsForUser errored:%s\n", err)
    return err
  }

  for i := 0; i < len(dbFFs); i++ {
    ff := dbFFs[i]  // this is just a string (feed name)
    fmt.Printf("following feed name:%s\n", ff)
  }
  return nil
}

func handlerUnfollow(s *state, cmd command, dbUser database.User) error {
  if len(cmd.args) < 1 {
    return errors.New("unfollow command expects 1 argument")
  }  
  url := cmd.args[0]
  ctx := context.Background()
  arg := database.DeleteFeedFollowParams {
    Url: url,
    Name: s.config.CurrentUsername,
  }

  _, err := s.db.DeleteFeedFollow(ctx, arg)
  if err != nil {
    return err
  }
  return nil  
}

func parseDate(s string) (time.Time, error) {
  var layouts []string = []string{time.RFC3339, time.RFC1123Z, time.RFC1123}
  var result time.Time
  for i := 0; i < len(layouts); i++ {
    result, err := time.Parse(layouts[i], s)
    if err == nil {
      return result, nil
    }
  }
  return result, errors.New("could not parse date")
}

func scrapeFeeds(s *state) {
  ctx := context.Background()
  feedsToFetch, err := s.db.GetNextFeedsToFetch(ctx)
  if err != nil {
    fmt.Printf("Error GetNextFeedsToFetch: %s\n", err)
    return
  }
  for i := 0; i < len(feedsToFetch); i++ {
    _, err = s.db.MarkFeedFetched(ctx, feedsToFetch[i].ID)
    feed, err := fetchFeed(ctx, feedsToFetch[i].Url)
    if err != nil {
      fmt.Printf("fetchFeed error:%s\n", err)
    }


    fmt.Printf("Fetched feed title: %s\n", feed.Channel.Title)
    for j := 0; j < len(feed.Channel.Item); j++ {
      item := feed.Channel.Item[j]
      uuid.New()
      desc := sql.NullString {
        String:item.Description,
        Valid:len(item.Description) > 0,
      }
      parsedDate, err := parseDate(item.PubDate)
      pubdate := sql.NullTime {
        Time: parsedDate,
        Valid: err == nil,
      }
      postParams := database.CreatePostParams {
        ID: uuid.New(),
        Title: item.Title,
        Url: item.Link,
        Description: desc,
        PublishedAt: pubdate,
        FeedID:feedsToFetch[i].ID,
      }
      _, err = s.db.CreatePost(ctx, postParams)
      if err != nil {
        // ignore errors from duplicate post URLs (we expect many of these)
        isDup := strings.Contains(err.Error(),
                                 "duplicate key value violates unique constraint")
        if !isDup {
          fmt.Printf("Error when creating post:%s\n", err)
        }
      }
      
      fmt.Printf("feed item title:%s\n", item.Title)
    }
  }
}

func handlerBrowse(s *state, cmd command, dbUser database.User) error {
  limit := 2
  if len(cmd.args) >= 1 {
    var err error
    limit, err = strconv.Atoi(cmd.args[0])
    if err != nil {
      return err
    }
  }

  params := database.GetPostsForUserParams {
    ID: dbUser.ID,
    Limit: int32(limit),
  }
  posts, err := s.db.GetPostsForUser(context.Background(), params)
  n := len(posts)
  for i := 0; i < n; i++ {
    p := posts[i]
    fmt.Printf("ID:%s\ncreated:%s\nupdated:%s\npublished:%s\ntitle:%s\nurl:%s\nfeedid:%s\n",
                p.ID, p.CreatedAt, p.UpdatedAt, p.PublishedAt.Time,
                p.Title, p.Url, p.FeedID)
    fmt.Printf("description:%s\n",  p.Description.String)
  }
  return err
}

func main() {
  argCount := len(os.Args)
  if argCount < 2 {
    fmt.Println("Usage: command arguments")
    os.Exit(1)
  }

	cfg, err := config.Read()
	if err != nil {
		fmt.Printf("main: config.Read() errored: %s\n", err)
		return
	}
	//fmt.Printf("CurrentUsername:%s DbUrl:%s\n", cfg.CurrentUsername, cfg.DbUrl)

  db, err := sql.Open("postgres", cfg.DbUrl)
  dbQueries := database.New(db)   // returns a *Queries


  var cmds commands
  cmds.m = make(map[string]func(*state, command) error)
  cmds.register("login", handlerLogin)
  cmds.register("register", handlerRegister)
  cmds.register("reset", handlerDeleteAllUsers)
  cmds.register("users", handlerUsers)
  cmds.register("agg", handlerAgg)
  cmds.register("addfeed", middlewareLoggedIn(handlerAddFeed))
  cmds.register("feeds", handlerFeeds)
  cmds.register("follow", middlewareLoggedIn(handlerFollow))
  cmds.register("following", middlewareLoggedIn(handlerFollowing))
  cmds.register("unfollow", middlewareLoggedIn(handlerUnfollow))
  cmds.register("browse", middlewareLoggedIn(handlerBrowse))

  var s state
  s.config = &cfg
  s.db = dbQueries
  var thisCommand command
  thisCommand.name = os.Args[1]
  thisCommand.args = os.Args[2:]
  err = cmds.run(&s, thisCommand)
  if err != nil {
    fmt.Printf("Command error: %s\n", err)
    os.Exit(1)
  }

  // go run . login alice

}