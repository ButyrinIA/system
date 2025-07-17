package models

import "time"

type Post struct {
	ID            string    `json:"id"`
	Title         string    `json:"title"`
	Content       string    `json:"content"`
	AuthorID      string    `json:"authorId"`
	AllowComments bool      `json:"allowComments"`
	CreatedAt     time.Time `json:"createdAt"`
}

type Comment struct {
	ID        string    `json:"id"`
	PostID    string    `json:"postId"`
	ParentID  *string   `json:"parentId"`
	AuthorID  string    `json:"authorId"`
	Content   string    `json:"content"`
	CreatedAt time.Time `json:"createdAt"`
}

type PaginatedComments struct {
	Comments   []Comment `json:"comments"`
	TotalCount int       `json:"totalCount"`
	NextCursor *string   `json:"nextCursor"`
}

type PaginatedPosts struct {
	Posts      []*Post `json:"posts"`
	TotalCount int     `json:"totalCount"`
	NextCursor *string `json:"nextCursor"`
}
