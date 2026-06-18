# AGENTS.md

You have full access to /home/megalith/CapperWeb and /home/megalith/CapsuleBuilder
When we make api CRUD (any change at all), lets make sure to update CapperWeb as well

CapDB lives in its own repo: <https://github.com/rickcollette/CapDB>

- To build/use CapDB, check it out (or update it) into /home/megalith/Capper/CapDB
  via `make capdb-fetch`. This is the only CapDB checkout the build should use
  (Makefile `CAPDB_DIR` defaults to ./CapDB, which is git-ignored).
- DO NOT touch /home/megalith/CapperVM/CapDB — that is the user's active working
  copy in another IDE. Never read from, write to, build against, or delete it.
