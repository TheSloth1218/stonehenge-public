import type { Metadata } from "next";
import { Toaster } from "sonner";
import { SiteTopBar } from "@/components/site-top-bar";
import "./globals.css";

export const dynamic = "force-dynamic";

export const metadata: Metadata = {
  title: "Stonehenge Control Authority",
  description:
    "Public regional asteroid-fragment tracking feed and institutional archive for the Stonehenge Control Authority.",
  icons: {
    icon: [{ url: "/media/sca-shield-logo.png", type: "image/png" }],
    shortcut: "/media/sca-shield-logo.png",
    apple: "/media/sca-shield-logo.png"
  }
};

export default function RootLayout({ children }: Readonly<{ children: React.ReactNode }>) {
  return (
    <html lang="en">
      <body>
        <SiteTopBar />
        {children}
        <div className="crt-mask" aria-hidden="true" />
        <Toaster theme="dark" richColors position="bottom-right" />
      </body>
    </html>
  );
}
