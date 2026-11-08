# Deployment

How each part goes live: website on Netlify or Vercel, API and worker on the Android phone behind a Cloudflare Tunnel, data in Supabase.

## Open questions (parked: decide when we deploy, the last step)

- Netlify or Vercel? Vercel's free plan is non-commercial only; Netlify's allows commercial use (Claude's suggestion: Netlify).
- Check the free-plan limits again before relying on them: Cloudflare 100 MB per request, Supabase pausing after about a week of inactivity.

## Before going live

- Accounts exist (step 6), but set `TOTAL_DAILY_CAP` first: signup isn't throttled, so the total cap is the real limit on the AI bill.
- Host the website on the API's own domain (`<domain>` and `api.<domain>`). The login cookie is SameSite=Lax, so a default netlify.app or vercel.app address breaks login.
- Cloudflare must send `CF-Connecting-IP`; login throttling uses it to tell students apart.
- Run `go run ./cmd/migrate` against the real database first. Supabase connection options: [server/internal/core/ARCHITECTURE.md](server/internal/core/ARCHITECTURE.md).
- Set `NEXT_PUBLIC_API_URL` when building the website. If it's missing, the live site quietly points at the visitor's own computer.
- Set `WEB_ORIGIN` on the API to the website's domain.
