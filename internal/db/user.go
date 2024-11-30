package db

import (
	_ "github.com/mattn/go-sqlite3"
)

// TODO: webauthn/passkeys
const createUserTable = `
create table if not exists users (
	id integer primary key autoincrement,
	username string not null unique
);
`

type User struct {
	Id       int64
	Username string
}

const insertUserQuery = `
insert into users(username)
values(?);
`

func (c *DbClient) CreateUser(username string) (User, error) {
	res, err := c.db.Exec(insertUserQuery, username)
	if err != nil {
		return User{}, err
	}
	userId, err := res.LastInsertId()
	if err != nil {
		return User{}, err
	}
	return User{userId, username}, nil
}

func (c *DbClient) GetUser(userId int64) (User, error) {
	row := c.db.QueryRow("select id, username from users where id = ?;", userId)
	var user User
	err := row.Scan(&user.Id, &user.Username)
	if err != nil {
		return User{}, err
	}
	return user, nil
}

func (c *DbClient) GetUserByUsername(username string) (User, error) {
	row := c.db.QueryRow("select id, username from users where username = ?;", username)
	var user User
	err := row.Scan(&user.Id, &user.Username)
	if err != nil {
		return User{}, err
	}
	return user, nil
}

