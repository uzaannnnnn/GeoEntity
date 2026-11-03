import { ApiErrorResponse } from '../types/entity';

export class ApiError extends Error {
  code: string;
  field?: string;
  status: number;
  details?: any[];

  constructor(status: number, code: string, message: string, field?: string, details?: any[]) {
    super(message);
    this.name = 'ApiError';
    this.status = status;
    this.code = code;
    this.field = field;
    this.details = details;
  }
}

export const API_BASE_URL = (import.meta.env.VITE_API_BASE_URL || '/api').replace(/\/+$/, '');

async function handleResponse<T>(response: Response): Promise<T> {
  if (response.status === 204) {
    return undefined as unknown as T;
  }

  const contentType = response.headers.get('content-type') || '';
  const isJson = contentType.includes('application/json');

  if (!response.ok) {
    if (isJson) {
      try {
        const errorJson = (await response.json()) as ApiErrorResponse;
        if (errorJson?.error) {
          throw new ApiError(
            response.status,
            errorJson.error.code || 'API_ERROR',
            errorJson.error.message || 'An unexpected API error occurred',
            errorJson.error.field,
            errorJson.error.details
          );
        }
      } catch (err) {
        if (err instanceof ApiError) {
          throw err;
        }
      }
    }
    throw new ApiError(
      response.status,
      'HTTP_ERROR',
      `Request failed with status ${response.status}: ${response.statusText}`
    );
  }

  if (!isJson) {
    throw new ApiError(response.status, 'INVALID_RESPONSE', 'Server did not return a JSON response');
  }

  return response.json();
}

export const apiClient = {
  async get<T>(path: string): Promise<T> {
    const response = await fetch(`${API_BASE_URL}${path}`, {
      method: 'GET',
      headers: {
        Accept: 'application/json',
      },
    });
    return handleResponse<T>(response);
  },

  async post<T>(path: string, body: unknown): Promise<T> {
    const response = await fetch(`${API_BASE_URL}${path}`, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        Accept: 'application/json',
      },
      body: JSON.stringify(body),
    });
    return handleResponse<T>(response);
  },

  async put<T>(path: string, body: unknown): Promise<T> {
    const response = await fetch(`${API_BASE_URL}${path}`, {
      method: 'PUT',
      headers: {
        'Content-Type': 'application/json',
        Accept: 'application/json',
      },
      body: JSON.stringify(body),
    });
    return handleResponse<T>(response);
  },

  async delete<T = void>(path: string): Promise<T> {
    const response = await fetch(`${API_BASE_URL}${path}`, {
      method: 'DELETE',
      headers: {
        Accept: 'application/json',
      },
    });
    return handleResponse<T>(response);
  },
};
