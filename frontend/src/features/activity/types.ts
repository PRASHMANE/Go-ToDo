export type ActivityType = 'todos.created' | 'todos.updated' | 'todos.completed' | 'todos.deleted';

export interface Activity {
  id: string;
  todo_id: string;
  type: ActivityType;
  title: string;
  occurred_at: string;
}
