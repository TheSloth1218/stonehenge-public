import { z } from "zod";

export const applicationRequestSchema = z.object({
  position: z.enum(["public-data-analyst", "archive-liaison", "civil-defense-coordinator"]),
  statement: z.string().trim().min(10).max(500),
  profileUrl: z.string().min(1).max(2048)
});

export type ApplicationRequest = z.infer<typeof applicationRequestSchema>;
