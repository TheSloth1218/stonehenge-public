"use client";

import { zodResolver } from "@hookform/resolvers/zod";
import { BriefcaseBusiness, X } from "lucide-react";
import { useEffect, useState } from "react";
import { useForm } from "react-hook-form";
import { toast } from "sonner";
import { Button } from "@/components/ui/button";
import { applicationRequestSchema, type ApplicationRequest } from "@/lib/training/review-schema";

export function StaffApplicationDialog() {
  const [open, setOpen] = useState(false);
  const { register, handleSubmit, reset, formState } = useForm<ApplicationRequest>({
    resolver: zodResolver(applicationRequestSchema),
    defaultValues: {
      position: "public-data-analyst",
      statement: "I would like to be considered for service with the Stonehenge Control Authority.",
      profileUrl: ""
    }
  });

  useEffect(() => {
    if (!open) return;
    const close = (event: KeyboardEvent) => event.key === "Escape" && setOpen(false);
    window.addEventListener("keydown", close);
    return () => window.removeEventListener("keydown", close);
  }, [open]);

  async function submit(values: ApplicationRequest) {
    const response = await fetch("/api/astro/apply", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(values)
    });
    const result = (await response.json()) as { submissionId?: string; status?: string; error?: string };
    if (!response.ok || !result.submissionId) throw new Error(result.error ?? "Application was not accepted.");
    toast.success("Application queued", {
      description: `Application ${result.submissionId.slice(0, 8)} is awaiting automated profile review.`
    });
    reset();
    setOpen(false);
  }

  return (
    <>
      <Button type="button" className="mt-8" onClick={() => setOpen(true)}>
        <BriefcaseBusiness className="size-4" /> Apply Now
      </Button>
      {open ? (
        <div
          className="fixed inset-0 z-[70] grid place-items-center bg-command-void/80 p-4"
          onMouseDown={(event) => event.target === event.currentTarget && setOpen(false)}
        >
          <section
            role="dialog"
            aria-modal="true"
            aria-labelledby="application-dialog-title"
            className="w-full max-w-lg border border-command-line/55 bg-command-void p-5 shadow-tactical"
          >
            <div className="flex items-start justify-between border-b border-command-line/25 pb-3">
              <div>
                <p className="font-mono text-[10px] uppercase text-command-line/70">FCU Civil Service Intake</p>
                <h2 id="application-dialog-title" className="font-acesHeavy text-2xl uppercase text-white">
                  Authority Application
                </h2>
              </div>
              <Button type="button" variant="ghost" size="icon" aria-label="Close application" onClick={() => setOpen(false)}>
                <X className="size-4" />
              </Button>
            </div>
            <form
              className="mt-4 grid gap-4"
              onSubmit={handleSubmit((values) =>
                void submit(values).catch((error) =>
                  toast.error("Application failed", {
                    description: error instanceof Error ? error.message : "Unknown error"
                  })
                )
              )}
            >
              <Field label="Position" error={formState.errors.position?.message}>
                <select {...register("position")} className="training-input">
                  <option value="public-data-analyst">Public Data Analyst</option>
                  <option value="archive-liaison">Archive Liaison</option>
                  <option value="civil-defense-coordinator">Civil Defense Coordinator</option>
                </select>
              </Field>
              <Field label="Short statement" error={formState.errors.statement?.message}>
                <textarea {...register("statement")} rows={3} className="training-input resize-none py-2" />
              </Field>
              <Field label="LinkedIn profile URL" error={formState.errors.profileUrl?.message}>
                <input type="url" inputMode="url" autoComplete="url" {...register("profileUrl")} className="training-input" />
              </Field>
              <p className="font-mono text-[10px] uppercase leading-4 text-command-calm/55">
                Submitted profiles are opened by the automated personnel review service.
              </p>
              <Button type="submit" disabled={formState.isSubmitting}>
                {formState.isSubmitting ? "Queueing Application" : "Submit Application"}
              </Button>
            </form>
          </section>
        </div>
      ) : null}
    </>
  );
}

function Field({ label, error, children }: { label: string; error?: string; children: React.ReactNode }) {
  return (
    <label className="grid gap-1 font-mono text-xs uppercase text-command-calm/70">
      {label}
      {children}
      {error ? <span className="text-command-hot">{error}</span> : null}
    </label>
  );
}
