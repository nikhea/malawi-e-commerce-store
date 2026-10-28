import type { Requester } from "../http";
import type {
  ForgotPasswordInput,
  LoginInput,
  OpStatus,
  RefreshInput,
  RegisterInput,
  ResetPasswordInput,
  TokenPair,
  VerifyEmailInput,
} from "../types";

/**
 * /api/v1/auth — all public, rate-limited 10 req/min/IP.
 */
export class AuthApi {
  constructor(private readonly request: Requester) {}

  register(input: RegisterInput): Promise<TokenPair> {
    return this.request("POST", "/auth/register", { body: input });
  }

  login(input: LoginInput): Promise<TokenPair> {
    return this.request("POST", "/auth/login", { body: input });
  }

  /** Rotate the token pair. Reusing a consumed token revokes the whole chain. */
  refresh(input: RefreshInput): Promise<TokenPair> {
    return this.request("POST", "/auth/refresh", { body: input });
  }

  /** 204 No Content. */
  logout(input: RefreshInput): Promise<void> {
    return this.request("POST", "/auth/logout", { body: input });
  }

  requestEmailVerification(input: ForgotPasswordInput): Promise<OpStatus> {
    return this.request("POST", "/auth/verify-email/request", { body: input });
  }

  verifyEmail(input: VerifyEmailInput): Promise<OpStatus> {
    return this.request("POST", "/auth/verify-email", { body: input });
  }

  requestPasswordReset(input: ForgotPasswordInput): Promise<OpStatus> {
    return this.request("POST", "/auth/password/forgot", { body: input });
  }

  resetPassword(input: ResetPasswordInput): Promise<OpStatus> {
    return this.request("POST", "/auth/password/reset", { body: input });
  }
}
