import { getSession } from "@/lib/auth";
import Image from "next/image";

type HomeProps = { searchParams: Promise<{ authError?: string; signedOut?: string }> };

export const dynamic = "force-dynamic";

export default async function Home({ searchParams }: HomeProps) {
  const [session, query] = await Promise.all([getSession(), searchParams]);
  const astroOrigin = process.env.ASTRO_PUBLIC_ORIGIN;
  if (!astroOrigin) throw new Error("ASTRO_PUBLIC_ORIGIN is required");

  return (
    <main>
      <header className="authority-bar">
        <Image src="/media/FCU_Flag.png" alt="Federation of Central Usea flag" width={48} height={32} />
        <span>
          <small>Federation of Central Usea</small>
          <strong>Central Identity Authority</strong>
        </span>
        <b>Unified Access / Dex</b>
      </header>

      <section className="login-shell" aria-labelledby="portal-title">
        <div className="identity-mark">
          <Image src="/media/sca-shield-logo.png" alt="Stonehenge Control Authority shield" width={124} height={124} priority />
        </div>
        <p className="eyebrow">FCU Secure Services</p>
        <h1 id="portal-title">Central Access Portal</h1>
        <p className="intro">One federation identity for authorized Stonehenge services.</p>

        {query.authError ? <p className="notice error">Authentication could not be completed. Verify the identity service configuration and try again.</p> : null}
        {query.signedOut ? <p className="notice">This application session has been closed.</p> : null}

        {session ? (
          <div className="session-panel">
            <div className="session-heading">
              <span className="status-light" aria-hidden="true" />
              <span>
                <small>Authenticated identity</small>
                <strong>{session.displayName}</strong>
              </span>
            </div>
            <dl>
              <div><dt>Account</dt><dd>{session.username}</dd></div>
              {session.email ? <div><dt>Directory mail</dt><dd>{session.email}</dd></div> : null}
              <div><dt>Identity source</dt><dd>FCU Dex</dd></div>
            </dl>
            <div className="actions">
              <a className="primary" href={astroOrigin}>Open Astro</a>
              <a className="secondary" href="/api/auth/logout">Sign out here</a>
            </div>
          </div>
        ) : (
          <div className="session-panel signed-out">
            <p>Continue to the federation identity service. Your Dex session will be recognized by every registered FCU application.</p>
            <a className="primary" href="/api/auth/login">Sign in with FCU Identity</a>
          </div>
        )}

        <footer>
          <span>OIDC Federation Channel</span>
          <span>OIDC signature verified</span>
        </footer>
      </section>
    </main>
  );
}
