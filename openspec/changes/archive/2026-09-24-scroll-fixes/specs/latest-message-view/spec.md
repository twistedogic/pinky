## MODIFIED Requirements

### Requirement: Yank to bottom on new content
The system SHALL keep the viewport pinned to the bottom of the rendered
message while the user is at the bottom, and SHALL release the pin the
moment the user scrolls up. While the pin is released, polling the agent
session and receiving new text SHALL NOT change the viewport's vertical
position — the user stays where they scrolled. The pin SHALL re-attach
automatically as soon as the user scrolls back to the bottom (so the next
poll resumes auto-follow). A new assistant message that replaces the
current one (a different `Text` from the prior `m.latest`) SHALL always
force-attach the pin so the new content is visible.

#### Scenario: Auto-follow while at the bottom
- **WHEN** the viewport's `YOffset` is at the bottom of the rendered
  message and a poll brings appended text to the same message
- **THEN** the viewport scrolls so the new last line sits at the bottom
  of the viewport

#### Scenario: Scrolled-up user is not yanked back
- **WHEN** the user has scrolled the viewport up (away from the bottom)
  and a poll brings appended text to the same message
- **THEN** the viewport's `YOffset` is unchanged; the user remains at
  their scroll position and the new text accumulates off-screen below

#### Scenario: Return to bottom re-attaches the pin
- **WHEN** the user scrolls back to the bottom (e.g. via `End`) and a
  subsequent poll brings appended text to the same message
- **THEN** the viewport scrolls so the new last line sits at the bottom
  of the viewport

#### Scenario: New message always scrolls to bottom
- **WHEN** a poll surfaces an assistant message whose `Text` differs
  from `m.latest.Text`
- **THEN** the viewport scrolls to the bottom of the new message
  regardless of the previous scroll position
