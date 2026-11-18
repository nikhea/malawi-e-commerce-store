import type { Requester } from "../http";
import type { ListUsersParams, Role, SetRoleInput, User, UserListResponse } from "../types";

/**
 * /api/v1/users — JWT required. Owners read themselves through `me`;
 * `adminGet` is gated server-side by the admin role. There is no public
 * `get(id)` (it leaked other accounts' email/role).
 */
export class UsersApi {
  constructor(private readonly request: Requester) {}

  me(): Promise<User> {
    return this.request("GET", "/users/me");
  }

  adminGet(id: string): Promise<User> {
    return this.request("GET", `/admin/users/${enc(id)}`);
  }

  /**
   * Admin user directory without envelope metadata. Same filters and
   * paging as `adminListPage`.
   */
  adminList(params?: ListUsersParams): Promise<User[]> {
    return this.request("GET", "/admin/users", { query: params });
  }

  /** Admin user directory plus pagination counters. */
  adminListPage(params?: ListUsersParams): Promise<UserListResponse> {
    return this.request("GET", "/admin/users", { query: params, raw: true });
  }

  adminSetRole(id: string, input: SetRoleInput): Promise<User> {
    return this.request("PATCH", `/admin/users/${enc(id)}/role`, { body: input });
  }

  adminSetRoleById(id: string, role: Role): Promise<User> {
    return this.adminSetRole(id, { role });
  }
}

function enc(value: string): string {
  return encodeURIComponent(value);
}
