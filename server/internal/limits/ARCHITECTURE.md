# Limits

Daily limits per student and in total, so the AI bill stays capped. "Today" is India time (Asia/Kolkata), worked out by Postgres.

## Per student

- 3 new lectures a day. A lecture counts when it's first added to the student's library, even if its note already exists and costs nothing.
- Adding a lecture already in the library doesn't count, and neither does Try again.
- The check and the insert happen in one transaction, holding the student's row locked, so two quick requests at 2 of 3 can't both pass.
- A refused lecture makes no job and no library row.

## In total

- `TOTAL_DAILY_CAP` counts new jobs made today, across every student. 0 means no cap.
- A lecture whose job already exists for that template passes: it costs nothing more.
- Checking takes a Postgres lock held until the transaction ends, so two new jobs can't both take the last place.
