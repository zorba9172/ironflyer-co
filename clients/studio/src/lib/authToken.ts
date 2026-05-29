const TOKEN_KEY = 'if-token';

export function getStoredAuthToken(): string | null {
  return localStorage.getItem(TOKEN_KEY);
}
