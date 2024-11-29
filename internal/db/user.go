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

const insertUserQuery = `
insert into users(username)
values(?);
`

func (c *DbClient) CreateUser(username string) (int64, error) {
	res, err := c.db.Exec(insertUserQuery, username)
	if err != nil {
		return 0, err
	}
	userId, err := res.LastInsertId()
	if err != nil {
		return 0, err
	}
	return userId, nil
}

type User struct {
	Id       int64
	Username string
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

