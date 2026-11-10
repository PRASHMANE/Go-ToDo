package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/PRASHMANE/Go-ToDo/services/todo-service/internal/model"
	"github.com/PRASHMANE/Go-ToDo/services/todo-service/internal/service"
)

// Compile-time proof that both repositories satisfy the service's interface.
var (
	_ service.TodoRepository = (*PostgresTodoRepository)(nil)
	_ service.TodoRepository = (*MemoryTodoRepository)(nil)
)

const todoColumns = `id, user_id, title, description, completed, priority, due_date, created_at, updated_at`

// likeEscaper escapes LIKE wildcards so a search for "100%" matches literally.
var likeEscaper = strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)

// PostgresTodoRepository stores todos in PostgreSQL.
type PostgresTodoRepository struct {
	db *pgxpool.Pool
}

// NewPostgresTodoRepository wraps a connection pool.
func NewPostgresTodoRepository(db *pgxpool.Pool) *PostgresTodoRepository {
	return &PostgresTodoRepository{db: db}
}

// scanTodo reads one row. pgx.Row is satisfied by both QueryRow results and Rows.
func scanTodo(row pgx.Row) (*model.Todo, error) {
	var t model.Todo
	var priority string
	if err := row.Scan(&t.ID, &t.UserID, &t.Title, &t.Description, &t.Completed,
		&priority, &t.DueDate, &t.CreatedAt, &t.UpdatedAt); err != nil {
		return nil, err
	}
	t.Priority = model.Priority(priority)
	// pgx returns times in the machine's local zone; the API always speaks UTC.
	t.CreatedAt, t.UpdatedAt = t.CreatedAt.UTC(), t.UpdatedAt.UTC()
	if t.DueDate != nil {
		due := t.DueDate.UTC()
		t.DueDate = &due
	}
	return &t, nil
}

// Create inserts a todo.
func (r *PostgresTodoRepository) Create(ctx context.Context, t *model.Todo) error {
	_, err := r.db.Exec(ctx,
		`INSERT INTO todos (`+todoColumns+`) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`,
		t.ID, t.UserID, t.Title, t.Description, t.Completed, string(t.Priority),
		t.DueDate, t.CreatedAt, t.UpdatedAt)
	if err != nil {
		return fmt.Errorf("insert todo: %w", err)
	}
	return nil
}

// GetByID returns one todo owned by userID.
func (r *PostgresTodoRepository) GetByID(ctx context.Context, userID, id string) (*model.Todo, error) {
	row := r.db.QueryRow(ctx,
		`SELECT `+todoColumns+` FROM todos WHERE id = $1 AND user_id = $2`, id, userID)
	t, err := scanTodo(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, model.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get todo: %w", err)
	}
	return t, nil
}

// List returns a filtered page of todos and the total match count.
func (r *PostgresTodoRepository) List(ctx context.Context, f model.ListFilter) ([]model.Todo, int, error) {
	// Build WHERE dynamically, but ONLY with placeholders. Values go in args.
	where := []string{"user_id = $1"}
	args := []any{f.UserID}

	if f.Completed != nil {
		args = append(args, *f.Completed)
		where = append(where, fmt.Sprintf("completed = $%d", len(args)))
	}
	if f.Priority != "" {
		args = append(args, string(f.Priority))
		where = append(where, fmt.Sprintf("priority = $%d", len(args)))
	}
	if f.Search != "" {
		args = append(args, "%"+likeEscaper.Replace(f.Search)+"%")
		n := len(args)
		where = append(where, fmt.Sprintf("(title ILIKE $%d OR description ILIKE $%d)", n, n))
	}
	whereSQL := strings.Join(where, " AND ")

	var total int
	if err := r.db.QueryRow(ctx, "SELECT COUNT(*) FROM todos WHERE "+whereSQL, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count todos: %w", err)
	}

	args = append(args, f.Limit, f.Offset)
	query := fmt.Sprintf(
		"SELECT %s FROM todos WHERE %s ORDER BY created_at DESC LIMIT $%d OFFSET $%d",
		todoColumns, whereSQL, len(args)-1, len(args))

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("list todos: %w", err)
	}
	defer rows.Close() // ALWAYS close rows, or the connection never returns to the pool

	var todos []model.Todo
	for rows.Next() {
		t, err := scanTodo(rows)
		if err != nil {
			return nil, 0, fmt.Errorf("scan todo: %w", err)
		}
		todos = append(todos, *t)
	}
	if err := rows.Err(); err != nil { // errors that happened during iteration
		return nil, 0, fmt.Errorf("iterate todos: %w", err)
	}
	return todos, total, nil
}

// Update saves all mutable fields of a todo.
func (r *PostgresTodoRepository) Update(ctx context.Context, t *model.Todo) error {
	tag, err := r.db.Exec(ctx,
		`UPDATE todos
		    SET title = $3, description = $4, completed = $5, priority = $6,
		        due_date = $7, updated_at = $8
		  WHERE id = $1 AND user_id = $2`,
		t.ID, t.UserID, t.Title, t.Description, t.Completed, string(t.Priority),
		t.DueDate, t.UpdatedAt)
	if err != nil {
		return fmt.Errorf("update todo: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return model.ErrNotFound
	}
	return nil
}

// Delete removes a todo.
func (r *PostgresTodoRepository) Delete(ctx context.Context, userID, id string) error {
	tag, err := r.db.Exec(ctx, `DELETE FROM todos WHERE id = $1 AND user_id = $2`, id, userID)
	if err != nil {
		return fmt.Errorf("delete todo: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return model.ErrNotFound
	}
	return nil
}
