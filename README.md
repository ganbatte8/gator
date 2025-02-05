
# Gator

This is my work following boot.dev's guided project "Build a Blog Aggregator".

It is a CLI program that can periodically send HTTP requests to various RSS feeds
and store some of the received data in a PostgreSQL database.
It has some multi-user functionality, different users can follow different feeds.

The project authors were inspired to name this project gator, because it's an aggregator.
It can aggregate different RSS feeds. If you're feeling an urge to send tomatoes right now, I think Lane Wagner is the person you want to reach out to.

# Installation

I'm not sure why you would subject yourself to the installation of this relatively uninteresting program, but I'll help you.

As a prerequisite, you will need to have installed the Go programming language and the PostgreSQL database program. I give some details on PostgreSQL in the section below.

My Go version was `1.23.4`. You can check yours with `go version`.

Once Go is installed, you can install this implementation of gator with
`go install github.com/ganbatte8/gator`.

`gator` should then be available as a command from anywhere on your command line if your PATH contains `$GOPATH/bin` (or `$GOBIN`). This installation process kind of assumes a Unix-based OS but it should be possible to set it up on Windows too, it should be different but similar, haha.


## Setting up the PostgreSQL database

PostgreSQL installation on different systems is similar but different.
I was on Manjaro (not recommended) when I wrote this program, which is an Arch Linux based distribution,
so the commands ended up being a little different than indicated in the course indeed.
My main reference for the installation and setup was this arch wiki page: https://wiki.archlinux.org/title/PostgreSQL.

PostgreSQL needs to be v15 or later. My version was `16.3`.

Once PostgreSQL is installed (supposedly via your distribution's package manager),
first you can check your installed version with `psql --version`.
Then you have to start the postgresql service (normally just once)
by running a command that presumably looks like this on Ubuntu:
```bash
sudo service postgresql start
```

In my case I figured I had to run two systemctl commands to start and enable the service:
```bash
systemctl start postgresql
systemctl enable postgresql
```

Then I was able to initialize the database with some series of commands like this:
```bash
# update password for user postgres, it will prompt you for it
sudo passwd postgres

su postgres
initdb -D /var/lib/postgres/data
```

The last command said that I can start the database server with this command (but I did not):
```bash
pg_ctl -D /var/lib/postgres/data -l logfile start
```
It appears that `pg_ctl` command was not necessary for me.
I did not research why, or what it does exactly.
But for the record, if you do try to run this command and it gives a file permission error,
it may be because the service wasn't started properly.

The next thing I did, while still being user `postgres`, is to run `psql`, which should successfully open, and I ran these PostgreSQL commands:
```sql
CREATE DATABASE gator;
\c gator   -- connect to the gator database
ALTER USER postgres PASSWORD 'postgres'; -- Linux only: change the database password
```

You can exit the `psql` CLI with `exit`.

## Creating the tables
Then you need to run the SQL "up migrations" that create the database tables.
The SQL code that creates these tables is contained in the sql files in `sql/schema`
in this project's repository, specifically the sections immediately preceded by `-- +goose Up` comments, ignoring the sections preceded by `-- +goose Down` comments.

It appears there is a bit of an oversight in the project's design:
if you installed the project with `go install`, you may not have the source files easily available, which makes running the migrations harder.
There are essentially two approaches for this: 1) access the source code, install goose and run the goose migrations with it. This is the way up and down migrations were run during development. 2) Simply extract and copy the SQL commands and run them yourself without Goose.

In the case of approach 1), if you used `go install`, the source code may be stored somewhere on your filesystem.
On my system it would be in `~/go/pkg/mod/github.com`. In the general case it can help to run `whereis gator` which will locate the compiled program (at least on Linux), which likely has a path similar to the source code. An alternative way to get the source code is to simply copy or clone this repo.
Once you have it, you can install `goose`:
``` bash
go install github.com/pressly/goose/v3/cmd/goose@latest
goose -version
```
Then `cd sql/schema` and run each up migration, by running this command five times:
```bash
goose postgres postgres://postgres:postgres@localhost:5432/gator up
```
Port `5432` is PostgreSQL's default port.

If you want to use approach 2): it is possible to give a file of sql commands to process by PostgreSQL with a command like this:
```bash
sudo -u postgres psql -d gator -f test.sql
```
For your convenience I have concatenated the SQL pieces of code that do all the up migrations into one file `make_tables.sql` at the root of the project so you don't have to grab the code from several different files yourself.
This makes future maintenance harder but I'm not expecting to touch this project in the future.

## Setting up the config file
Create a json file `.gatorconfig.json` in your home directory. Put this JSON content inside:
```json
{"db_url":"postgres://postgres:postgres@localhost:5432/gator?sslmode=disable",
"current_user_name":"kahya"}
```
The gator program will use the `db_url` connection string to make requests to the PostgreSQL server. You can replace the current username "kahya" with any string you want, but it will be replaced as you use the CLI anyway; in particular you will first need to run a gator command to register a username into the database as described below.


# Commands

The full list of gator commands is visible in the `main()` function of the `main.go` file.

The general syntax to use the CLI is `gator commandname arg0 arg1 ...`.
I'm going to describe most of them, roughly in the order that you would use them.

`gator register your_name`
This creates a user with name `your_name`, recorded in the database. It will also set the config file's current user name to that name.

`gator login your_name`
This tries to fetch a user record in the database by matching `your_name`, and if successful it will set the config file's current user name to that name.

`gator users`
This lists all user names in the database. It also indicates which one is the current username.

`gator addfeed feedname url`
Add an RSS feed in the database. `feedname` can be any name you choose. `url` should match a real RSS link, some examples being
- TechCrunch: `https://techcrunch.com/feed/`
- Hacker News: `https://news.ycombinator.com/rss`
- Boot.dev Blog: `https://blog.boot.dev/index.xml`

Note that the database associates a unique `user_id` to each feed record.
This is kind of weird, especially considering that we also have a many-to-many relationship between feeds and users implemented by the `feed_follows` table.
If I remade this I think I would keep the join table and remove the `user_id` foreign key in the feeds table. It is also possible I didn't follow the course instructions properly or misinterpreted some step or something.

`gator feeds`
List all the feeds and the associated user names (but those associations are kind of meaningless, as we just mentioned)

`gator follow url`
Make the current user follow the feed at the current url.
This assumes the feed already exists in the database (which is perhaps inconvenient, unless you want to be very intentional about adding records in the feeds table). It will add a record in the `feed_follows` table, effectively linking a `users` record with a `feeds` record.

`gator unfollow url`
This undoes the previous command.

`gator following`
Lists the feed names that the current user is following.

`gator agg time_between_reqs`
This sends HTTP GET requests to the feeds followed by the current user, periodically with a period specified by `time_between_reqs`. An example period is `60s` to send a GET request to each feed every minute (please do not spam/DOS the servers). It receives and stores certain xml fields of the responses into the `posts` table of the database.

`gator browse lim`
This prints some information (notably title and description) of the posts stored in the database (previously fetched with `gator agg`), for the feeds followed by the current user. The posts are sorted by publication date, most recent first. The command prints up to `lim` (an integer) number of posts. `lim` is an optional argument with default value `2`.


## Program usage in a nutshell
First you register a user if you don't have any,
make sure you're logged in as the user you want to be,
add a feed with `gator addfeed feedname url`,
then follow it with `gator follow url` (you must add the feed before following it),
scrape the feeds you're following with `gator agg 60s`,
and print them with `gator browse lim`.



