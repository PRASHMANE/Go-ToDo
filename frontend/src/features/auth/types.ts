export interface User {
  id: string;
  email: string;
  name: string;
  created_at: string;
}

export interface LoginInput {
  email: string;
  password: string;
}

export interface RegisterInput extends LoginInput {
  name: string;
}

export interface LoginResponse {
  access_token: string;
  token_type: 'Bearer';
  expires_at: string;
  user: User;
}
