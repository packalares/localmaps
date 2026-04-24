import { redirect } from "next/navigation";

/**
 * Admin landing → /admin/regions. Regions is the primary admin
 * surface; other tabs (Settings, Jobs) own their own routes.
 */
export default function AdminIndex() {
  redirect("/admin/regions");
}
