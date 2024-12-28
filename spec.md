# Textbook Protocol Specification

### Motivation

I've noticed than many educational platforms today tend to share the idea
that it is helpful to split up content into smaller chunks, and to mix
in interaction/questions with the content.

I've really enjoyed using several of these platforms, and they are
growing more popular. This is great! However, each of these platforms
are bespoke, and their content is locked away. Ideally, as more of these
platforms pop up, and more people find these tools effective for teaching/learning,
the content that people create should not be tied to a particular platform.
A protocol for this content could make this easier, since it would allow
people to create chunks of content, and they could then be easily used
on any program that supports the protocol.

In the future, it would also be really cool to weave together content
from different platforms to teach particular topics. For example, to
teach some cryptography, a student might need to understand some
number theory, and I might want to pull in a few modules from Math
Academy, since they've already made great content on that. And then
it's not hard to imagine that a program could automatically stitch
together best content from different teachers and platforms to
create a sequence of content that is tailored towards a specific
question or topic. Without a protocol, making something like
this is probably pretty tough.

It's probably a bit premature to actually try to get people on
board with this, but I can start developing this in the meantime
for my own use case, and see how things evolve.

### Overview

Goal: Simple yet expressive

Right now the protocol is just:
- Markdown for general content
- Latex for math
- Markdown comments to denote different "blocks"
    - Content blocks - just some content
    - Question blocks - for now just multiple choice question,
    but in the future we should be able to incorporate all kinds
    of interaction.

### Example

Here is an example "module" in the protocol:

```markdown
---
title: Example Module
---

[//]: # (content)

# Introduction

This is an example module. It's just a simple markdown file.

[//]: # (question: multiple_choice)

What is $5^3$?

[//]: # (choice)
15
[//]: # (choice)
25
[//]: # (choice)
75
[//]: # (choice correct)
125

[//]: # (explanation)

The answer is 125 because $5^3 = 5 * 5 * 5 = 125$.

[//]: # (content)

# Conclusion

That's it for today's lesson!

```


