import type { Metadata } from "next";
import "./globals.css";

export const metadata: Metadata = {
  title: "FCU Identity Service",
  description: "Federation of Central Usea single sign-on portal"
};

export default function RootLayout({ children }: Readonly<{ children: React.ReactNode }>) {
  return (
    <html lang="en">
      <body>
        {children}
        <div className="crt-mask" aria-hidden="true" />
      </body>
    </html>
  );
}
