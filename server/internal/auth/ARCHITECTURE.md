# Auth

Our own accounts: signup, login, logout and sessions. Supabase is only the database. A user's template lives here too, so there's no separate users package: it would hold one table and two queries.

## Accounts

- Email is trimmed and lower-cased, then stored and compared that way. Only plain addresses pass.
- Passwords are 8 to 128 characters, hashed with argon2id (19 MiB, 2 passes, 1 thread, from OWASP's advice). The settings are kept in each hash, so they can be raised later.
- At most 4 hashes run at once, so a burst of logins waits instead of using up the phone's memory.
- Each student has a template, `neet` or `general`, picked at signup. Changing it only affects lectures added afterwards.
- No Google sign-in, no email check and no password reset: there's no email service yet.

## Sessions

- A session is a random token from `crypto/rand`, sent in the `warden_session` cookie. The database keeps only its SHA-256, so a copy of the table can't log anyone in.
- Tokens are looked up by their hash. Nobody can choose what a hash starts with, so how long the lookup takes says nothing useful about the token.
- A session lasts 30 days. Logout deletes it. Expired ones are cleared out when that student next logs in.

## Login

- A wrong email and a wrong password give the same answer, and take the same time: an unknown email is still checked against a hash.
- After 5 wrong passwords for one email, or 20 from one address, within 15 minutes, login answers 429 until the window passes, even with the right password.

shortcut: the wrong-password counts live in the API's memory, so a restart forgets them and two API processes wouldn't share them. Move them to Postgres if the API ever runs as more than one process.
