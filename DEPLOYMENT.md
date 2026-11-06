# Deployment

How each part goes live: website on Netlify or Vercel, API and worker on the Android phone behind a Cloudflare Tunnel, data in Supabase.

## Open questions (parked: decide when we deploy, the last step)

- Netlify or Vercel? Vercel's free plan is non-commercial only; Netlify's allows commercial use (Claude's suggestion: Netlify).
- Check the free-plan limits again before relying on them: Cloudflare 100 MB per request, Supabase pausing after about a week of inactivity.

## Before going live

- The API has no login. It must stay on 127.0.0.1 until step 6.
- Run `go run ./cmd/migrate` against the real database first. Supabase connection options: [server/internal/core/ARCHITECTURE.md](server/internal/core/ARCHITECTURE.md).
- Set `NEXT_PUBLIC_API_URL` when building the website. If it's missing, the live site quietly points at the visitor's own computer.
- Set `WEB_ORIGIN` on the API to the website's domain.
