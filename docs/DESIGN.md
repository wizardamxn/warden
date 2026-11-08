# Warden design guide

Read this before building or changing any screen. It combines the study research (how people learn from video) with the visual research (what makes a UI look made by a person, not by AI). Researched 2026-10-25. Sources at the bottom.

## The idea in one line

Light at the end of the tunnel. A warm, optimistic paper page lit by one soft sunny glow, with a quiet reading column next to the video. A yellow highlighter follows the video as it plays. Nothing on screen that doesn't help you read, watch or test yourself.

(Chosen 2026-10-25: light, not dark. The study research found no reading difference between light and dark, so this is a feel choice.)

The page is a NEET student's desk (chosen 2026-10-31). Cream paper plus a serif is a common AI-made look, so the character comes from things every NEET student knows: the OMR sheet, a yellow highlighter, a blue pen and a red pen.

## 1. What the study research says

Fairly solid:

- Testing yourself beats rereading. Flashcards and quizzes help memory far more than reading notes again; highlighting and rereading do little.
- Watching a video feels like learning more than it is. So the end of the notes must lead straight into testing yourself.
- Spacing helps: coming back days later beats one cram.
- Clutter hurts learning. Anything that doesn't help splits attention. Decorative pictures and background music make learning worse.
- Keep related things together: notes beside the video, matching what's playing.
- Make clear where attention should go: short sections, clear headings, TL;DR first.
- Line length about 60–75 characters.

Weak or mixed:

- Questions that pause the video mid-way: studies disagree. Not used.
- Dark vs light for reading: no good study either way.
- Streaks: bring people back, but people chase the streak and quit when it breaks. Not in v1.

## 2. What the visual research says

Why AI-made UIs all look the same (the "slop test": if someone would guess "AI made this", it fails):

- Default fonts (Inter, Roboto, system), purple/blue gradients, cyan or amber "glow" on navy.
- Everything wrapped in rounded cards, cards inside cards, identical card grids.
- Pill buttons everywhere, tiny ALL-CAPS labels with wide letter spacing, an icon above every heading.
- Centred hero headline over an input. Every button looks primary.
- Pure black and pure white. Generic shadows. Animations scattered everywhere.
- Hollow copy ("Supercharge your learning").

What premium products do instead (Linear, Readwise Reader, Stripe Press):

- Not every element gets equal weight. Navigation steps back (Linear dims its sidebar); the content is the brightest thing on screen.
- "Structure should be felt, not seen" (Linear): space and type do the separating, not boxes and borders.
- Strict type discipline: few sizes, one clear scale, tight tracking on big headings.
- Warm, slightly desaturated neutrals instead of cold blue-grey (Linear's latest refresh moved warmer). Never pure black or white.
- In dark mode, raise surfaces by making them a little lighter, not with shadows. Desaturate accent colours.
- Reading surfaces (Readwise Reader): about 65 characters per line, generous margins, no toolbars while reading, notes in the margin so the text column is never interrupted. Highlight colours look like ink on paper, not neon.
- One signature moment, done well, instead of many small effects.

## 3. Rules for Warden

### Look

- Background: warm paper (`#FBF7EE`), not pure white.
- The glow: one soft sunny light (`#FFF1BF`, radial, fading to the paper) per screen, placed where attention should go: behind the headline, the video, the score. It is the "light at the end of the tunnel". Never a gradient wallpaper, never more than one.
- Shade (`#F0E8D8`): the page edges in focus mode, when the light narrows.
- Text: three levels only. Primary `#2B2620` (warm ink, not pure black). Secondary `#6B6255`. Hairlines and decoration `#E3DACA` (never for text). Input underline `#B8AD99`.
- Highlighter: yellow `#F6D364`, used semi-transparent under the text like a real marker. Live highlight, correct answers, the final score.
- Primary button (one per screen): warm gold `#EDBE45` with ink text.
- Blue pen `#1F4FA3`: what the student marks, like a filled OMR bubble.
- Red pen `#C7261F`: wrong answers and the teacher's exam marks in the margin ("Asked 2018, 2024"). Only shown when the teacher really said it.
- Flashcards: a bright index card (`#FFFDF8`) with faint blue rules and one red line, softly lifted off the paper.
- Video frame: stays dark (`#1F1C18`); videos are dark.
- All colours are named theme values in one place. Dark mode (2026-11-08) is the same desk at night: warm near-black paper, a dim lamp for the glow, cards a little lighter than the desk instead of shadowed, pens lighter and less saturated, and a darker highlighter so light ink stays readable on it.

### Type

- Reading and headings: Newsreader (a serif built for screen reading).
- Interface (buttons, labels, timestamps): Instrument Sans. Not Inter.
- Scale (ratio 1.25): 13, 16, 19, 24, 30, 38, 48 px. Nothing in between.
- Notes body: 19 px, line height 1.6, max 62 characters per line. (If dark mode comes later: slightly more line height and a touch of letter spacing.)
- Big headings: tight letter spacing (about -1%).
- Labels in sentence case. No tiny ALL-CAPS.
- Timestamps use even-width digits so they line up.

### Layout

- No cards. Separate with space and the odd hairline.
- Left-aligned and asymmetric. Nothing centred except the flashcard.
- Spacing on an 8 px grid; vary it to create rhythm (big gaps between sections, small within).
- Timestamps hang in the left margin of the notes, like margin notes in a textbook.
- Navigation is quiet and small. The video and the notes are what the light falls on.
- One primary button per screen. Everything else is a text link or quiet button.

### Motion

- Only transform and opacity. Smooth slow-down easing, no bounce.
- About 0.15–0.4 s. Off when the device asks for less motion; nothing depends on it.

### The signature moments: the light and the highlighter

- As the video plays, a yellow highlighter stroke sweeps under the heading of the section being played. When the video moves on, the stroke moves with it.
- The same stroke marks a correct quiz answer and the final quiz score. The score also sits in the glow (that is the small celebration).
- While notes are being made, the glow grows a little with each finished step: the light gets closer.
- Focus mode narrows the light: navigation disappears, the page edges fall into soft shade, and every section except the playing one fades, so only the video and the playing section stay lit.
- Flashcards are a real-looking index card that flips.
- The quiz is an OMR sheet, and this is where the boldness goes: answers fill in as bubbles, marked +4 and −1 like the real paper.
- Revise is a pile of cards waiting from older lectures, not a list. It drops onto the desk when opened, and Next puts a card aside.
- Moving between pages, the old page steps back and the new one rises into place; a lecture's still grows into its player. This and the quiz marking are the motion budget: no more effects without a reason.

### Copy

- Plain and specific. Say what happens ("Paste a YouTube link. Get notes beside the video, then test yourself.").
- No slogans, no "AI-powered", no exclamation marks.

## Sources

Study research:

- [Mayer – Multimedia Principles (2020)](https://ugc.futurelearn.com/uploads/files/7d/d6/7dd6188d-c343-4311-b064-ac98d2c95abc/Multimedia_Principles._R._E._Mayer__2020.pdf)
- [EdTech Books – Principles of Multimedia Design](https://edtechbooks.org/designing_transmedia_learning_environments/principles_of_multimedia_design)
- [Lollypop – UX in education](https://lollypop.design/blog/2025/july/ux-design-in-education-student-engagement/)
- [Lecturio – Retrieval practice](https://lecturio.com/pulse/retrieval-based-learning-strategies-in-medical-education)
- [Baymard – Line length](https://baymard.com/blog/line-length-readability)
- [Optimal Line Length – literature review](https://journals.uc.edu/index.php/vl/article/view/5765)
- [Mind wandering in video learning](https://ojs.aut.ac.nz/pjtel/article/view/178)
- [Fluency and illusion of knowing (TV)](https://sonar.rero.ch/global/documents/107972)
- [Decision Lab – Streak creep](https://thedecisionlab.com/insights/consumer-insights/streak-creep-the-perils-of-too-much-gamification)
- [Glasp vs Eightify](https://glasp.ai/youtube-summary-vs-eightify), [Glasp vs NoteGPT](https://glasp.ai/youtube-summary-vs-notegpt)

Visual research:

- [Impeccable – Anti-patterns and the slop test](https://mintlify.wiki/pbakaus/impeccable/concepts/anti-patterns)
- [Noqta – AI design slop, overused patterns](https://noqta.tn/en/blog/ai-design-slop-overused-ui-patterns-fix-2026)
- [Superdesign – Why AI design looks generic](https://www.superdesign.dev/blog/why-ai-design-looks-generic)
- [Repaint – A picture is worth a thousand tokens](https://repaint.com/blog/picture-is-worth-a-thousand-tokens)
- [Linear – Behind the latest design refresh](https://linear.app/blog/behind-the-latest-design-refresh)
- [Linear – How we redesigned the Linear UI](https://linear.app/blog/how-we-redesigned-the-linear-ui)
- [Readwise Reader design breakdown](https://blakecrosley.com/guides/design/readwise-reader)
- [OpenDesigner – Stripe, Linear, Vercel design systems](https://opendesigner.io/de/blog/recreating-stripe-linear-vercel-design-systems-with-design-md)
- [Uxcel – Dark mode principles](https://uxcel.com/blog/12-principles-of-dark-mode-design-627)
- [Atmos – Dark mode best practices](https://atmos.style/blog/dark-mode-ui-best-practices)
- [Impeccable – Typography reference](https://cdn.jsdelivr.net/npm/@nqlib/nqui@0.6.3/docs/nqui-skills/impeccable/reference/typography.md)
- [Frog – Signature moments](https://www.frog.co/designmind/making-the-most-of-your-signature-moments)
- [UXPin – Delightful interaction design](https://www.uxpin.com/studio/?p=8061)

Most sources are design blogs and third-party breakdowns, not peer-reviewed research. Colour values are our own choices, not copied from any product.
