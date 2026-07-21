## What

Closes #

## Golden rules

Which of `CLAUDE.md`'s golden rules does this touch, and how does it stay inside them?

- [ ] Adds no resource with an hourly floor (or an ADR explains why)
- [ ] Lambda stays out of the VPC
- [ ] No Globus HTTP calls outside `internal/auth`
- [ ] No user Transfer token reaches ephemeral compute
- [ ] Context threaded; no stdlib default logger, no `fmt.Println` in library code

## Verification

How was this checked against real AWS, not just unit tests?
