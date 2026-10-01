import { createContext, useContext, useEffect, useState, type ReactNode } from "react";
import type { AuthRecord } from "pocketbase";
import { pb } from "./pocketbase";

const AuthContext = createContext<AuthRecord | null>(null);

export function AuthProvider({ children }: { children: ReactNode }) {
  const [auth, setAuth] = useState<AuthRecord | null>(pb.authStore.record);

  // authStore is the source of truth (persisted to localStorage, updated by realtime)
  useEffect(() => pb.authStore.onChange((_token, record) => setAuth(record)), []);

  return <AuthContext.Provider value={auth}>{children}</AuthContext.Provider>;
}

export function useAuth() {
  return useContext(AuthContext);
}
