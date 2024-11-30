package db

import (
	"database/sql"

	_ "github.com/mattn/go-sqlite3"
)


const createContentTable = `
create table if not exists content (
	id integer primary key autoincrement,
	content text not null
);
`

const createContentBlockTable = `
create table if not exists content_blocks (
	id integer primary key autoincrement,
	block_id integer not null unique,
	content_id integer not null,
	foreign key (block_id) references blocks(id) on delete cascade,
	foreign key (content_id) references content(id) on delete cascade
);
`

type Content struct {
	Id      int
	Content string
}

func NewContent(id int, content string) Content {
	return Content{id, content}
}

const insertContentQuery = `
insert into content(content)
values(?);
`

func InsertContent(tx *sql.Tx, content string) (int64, error) {
	res, err := tx.Exec(insertContentQuery, content)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

const insertContentBlockQuery = `
insert into content_blocks(block_id, content_id)
values(?, ?);
`

func InsertContentBlock(tx *sql.Tx, blockId int64, content string) error {
	contentId, err := InsertContent(tx, content)
	if err != nil {
		return err
	}
	_, err = tx.Exec(insertContentBlockQuery, blockId, contentId)
	if err != nil {
		return err
	}
	return nil
}

const updateContentQuery = `
update content
set content = ?
where id = ?;
`

func UpdateContent(tx *sql.Tx, contentId int64, content string) error {
	_, err := tx.Exec(updateContentQuery, content, contentId)
	return err
}


const getContentForBlockQuery = `
select c.id, c.content
from content c
join content_blocks cb on c.id = cb.content_id
where cb.block_id = ?;
`

func (c *DbClient) GetContentFromBlock(blockId int) (Content, error) {
	contentRow := c.db.QueryRow(getContentForBlockQuery, blockId)
	content := Content{}
	err := contentRow.Scan(&content.Id, &content.Content)
	if err != nil {
		return Content{}, err
	}
	return content, nil
}
