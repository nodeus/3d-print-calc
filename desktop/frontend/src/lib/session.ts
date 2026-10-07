import { writable } from 'svelte/store';
import { wails, type UserDTO } from './wails';

// Текущая сессия. null — гость (только чтение).
export const user = writable<UserDTO | null>(null);

export async function refreshUser(): Promise<void> {
  try {
    user.set(await wails.currentUser());
  } catch {
    user.set(null);
  }
}

export function canWrite(u: UserDTO | null): boolean {
  return !!u && (u.role === 'admin' || u.role === 'manager');
}

export function isAdmin(u: UserDTO | null): boolean {
  return !!u && u.role === 'admin';
}
