import type { Requester } from "../http";
import type { DashboardOverview } from "../types";

/** Admin-only dashboard reads. Requires a JWT carrying the admin role. */
export class DashboardApi {
  constructor(private readonly request: Requester) {}

  /** GET /admin/dashboard/overview — whole-store snapshot. */
  overview(): Promise<DashboardOverview> {
    return this.request("GET", "/admin/dashboard/overview");
  }
}
