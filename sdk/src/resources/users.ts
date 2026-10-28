import type { Requester } from "../http";
import type { Role, SetRoleInput, User } from "../types";

/**
 * /api/v1/users — JWT required. `get` is readable by any authenticated user;
 * admin endpoints are gated server-side by the admin role.
 */
export class UsersApi {
  constructor(private readonly request: Requester) {}

  me(): Promise<User> {
    return this.request("GET", "/users/me");
  }

  get(id: string): Promise<User> {
    return this.request("GET", `/users/${enc(id)}`);
  }

  adminGet(id: string): Promise<User> {
    return this.request("GET", `/admin/users/${enc(id)}`);
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
