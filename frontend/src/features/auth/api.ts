import { apiClient } from '../../lib/api-client';
import type { LoginInput, LoginResponse, RegisterInput, User } from './types';

export const authApi = {
  async register(input: RegisterInput): Promise<User> {
    const { data } = await apiClient.post<User>('/auth/register', input);
    return data;
  },
  async login(input: LoginInput): Promise<LoginResponse> {
    const { data } = await apiClient.post<LoginResponse>('/auth/login', input);
    return data;
  },
  async me(): Promise<User> {
    const { data } = await apiClient.get<User>('/auth/me');
    return data;
  },
};
