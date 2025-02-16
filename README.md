# Noobular

A website to let people create courses and learn from them.

Premise: There are several bespoke learning platforms out there, e.g.
Duolingo, Brilliant, MathAcademy, etc. that share a lot of similar features.
They have some set of exercises, they have a bunch of courses,
keep track of your progress, give you rewards for streaks, have
leaderboards/leagues to rank up on, diagnostic tests to place you, etc.
What if we allowed anyone to create courses and let some shared infrastructure
take care of the gamification, logistics, and discovery?

Premise cont: The internet allows information to be a lot more flexible.
So textbooks can now be interactive, chopped up into smaller pieces,
and mixed with different forms of media. These latest platforms are
just the natural evolution of being able to curate content on the
internet specifically for learning things effectively. This
evolution will only continue. I believe that certain pieces of content
can really make a difference in how someone learns something (and whether
they continue learning!), and so this platform stands to honor those
noble teachers.

Realistically this will just be another side project, but it's
still fun to give it a motivation.

More notes:

This is simply for fun/convenience, users should always be able
to take their data elsewhere. Ideally users input their
modules as markdown to make this even easier.

The heart of this app is the content system. It should be simple
but expressive.

The kind of flow that brilliant/math academy use is really similar
to how I like to give lectures. It's harder to do open-ended questions
here with meaningful feedback, but the most part, I always like to
introduce concepts, and give a quick check/question to have some
back and forth between me/the material and the student/audience.

The heart of this app should be a protocol for expressing educational
content, i.e. markdown + a little syntactic sugar to denote multiple
choice questions, short answer, etc. The rest of the app is simply an
interface for this. Main features: ui to input/store/share this content,
rendering this content, interacting with this content, tracking progress
(i.e. persisting interactions with the content). Keep it lean!

Eventually it would be cool for others to build other things on top
of this protocol, like an offline version.

Goal: effectively teach people things
- Need to sincerely tackle all of the important understandings about learning,
outlined incredibly in https://x.com/justinskycak/status/1858012912557633800.
    - As noted in the tweet, only exceptionally motivated students will
    leverage a platform on their own. Most students need an adult/other
    person to actively hold them accountable.
- One really cool thing about math academy is the way they 1. recognize you're
weaknesses, and then 2. they can provide you with a lesson/review from a
previous course that will be helpful to strengthen that. The *connectedness*
of the content across modules + courses is really brilliant. I've been thinking,
for some of the cryptography stuff, it would be great if I could surface
some math academy modules that would be helpful prerequisites to understanding
certain things. Otherwise I need to recreate the content just for this module,
when math academy has already made the exact thing I needed and probably 10x
better. This is how a protocol could be really powerful.
    - Imagine in the future, instead of bespoke platforms and courses,
    you have a program with access to a vast network of content, and it
    can stitch together the best pieces of content from various topics
    into a sequence tailored to something that you're curious about. This
    is the direction math academy is heading in with the connectness of its
    different courses. It's super exciting, and within reach!

Tech stack
- Frontend: htmx
- Backend: go
- Database: sqlite (https://github.com/mattn/go-sqlite3)
- Hosting: raspberry pi + cloudflare

Ideas for courses I would like:
- Electronics - schematic builder/si mulator + exercises/puzzles
- Cryptography - exercises/puzzles

Security things to remember if someone besides me ever uses this:
- HTTPS
- Sql injection
- https://htmx.org/essays/web-security-basics-with-htmx/
- Sanitize markdown
- Full scan of all user generated content

Who might use this?
- Me - put together some content to spread knowledge to coworkers, friends, family
- Teachers - assign modules as in-class activity or homework
    - Many features would need to be added, but probably not too hard to support

Gamification:
- I am not super sure how helpful most gamification features are. I and most people I know never really cared to use X game items in duolingo. Though, it was a decent incentive to complete friend quests, and october challenges. But I feel like these are kind of gimmicky. Although they be sort of effective at getting you to do a little extra in a single session, I eventually kinda felt like they were not that meaningful.
- I think something that is meaningful is user stats, and public profiles where others can see your activity. There's a couple valuable aspects:
    - It's actually interesting feedback. I personally have wondered to see Math Academy's evaluation of my skill level in various topics based on how much time I spent doing certain problems, as well as aggregate stats of how well on did on various things. It is just a cool thing to learn about yourself, like "hey for some reason I got these concepts faster than others." And if I want to, I can use that to identify weak spots.
    - They are numbers to optimize over time. I feel like this is a pretty simple but effective form of gamification without getting too gimmicky. I generally want to see my   correct/incorrect ratio improve over time. As long as these metrics are chosen well, they are incentivizing the right improvement.
    - People like to signal to others stuff like this. Similar existing examples: spotify wrapped, github profiles. Your public activity log is a representation of you/how you spend your time. People like to show others who they are, and it's a testament to people's hard work.

Feedback from math academy:
- I feel like sometimes I pass review when I actually kind of forgot what the definition of something is
- Sometimes the questions are not a good proxy for understanding. Sometimes I just kind of click
through a proof, or I can kind of deduce what certain answers are from the options provided. Whereas
I really never have to remember the template for myself..?
    - I want to be extremely thoughtful about doing anything gimmicky with AI. But I think this could
    be an interesting usage. Where a teacher gives a rubric for what they want from an answer, and a
    student can be asked to provide a fairly open ended answer, and the AI can give feedback according
    to the rubric.
- The spaced repetition for some things is too spaced where i actually forgot stuff.
This might be sorta limited/unsolvable, but yea.
- I just got 3 of the same question in a row for graphing arcsin.

Reasons I may want to consider integrating an LLM
- Open ended answers
    - Sometimes questions with fixed choices are not a good enough proxy of understanding.
    More open ended answers are often much better proxies, but they are harder to grade automatically.
    - Two cases
        - Simple fill in the blank - when an answer is a word or couple words, you don't always want to
        just mark it wrong for not being exact, i.e. Satoshi and satoshi nakamoto are semantically the same.
        - Actually open-answered questions - often in math academy I'll fill out a proof template. Here
        I never really need to recall how the proof is formed myself, and instead just select dropdowns.
        Imagine if I was asked to form the proof myself, and an AI can grade whether it included the
        required elements/followed the right form.
    - Students would always have the option to submit for manual review. If they catch an error with the
    AI, they should get bonus points, and that case should be used to further tune it.

Thinking about why I didn't really get that much from fastai
- I really wonder the depths of interactivity we can venture

Restatement of core principles of this app
- Content protocol for internet-native textbooks to last decades
- Leveraging decades of evidence on what makes people effectively learn
- Extreme technical simplicity
    - Important: Use only a minimum of excellent abstractions, implement only a minimum of excellent features
- Goal: give people tools to teach and learn more effectively
