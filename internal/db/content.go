package db

import (
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

const getExplanationContentQuery = `
select c.id, c.content
from explanations e
join content c on e.content_id = c.id
where e.question_id = ?;
`

const insertContentQuery = `
insert into content(content)
values(?);
`

const updateContentQuery = `
update content
set content = ?
where id = ?;
`

