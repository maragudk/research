# Glimmerblob

A mobile-first three.js pocket pet: a Tamagotchi-like creature, but quirkier and
sparklier. Everything is in one self-contained `index.html`. It loads three.js r128
from cdnjs and the fonts from Google Fonts.

## Playing

- **Hatch** the egg by tapping it three times. Every egg gets a random name (tap the
  name to rename it), a colour, a secret favourite snack and one snack it hates.
- **Boop** it with a tap. If you boop it seven times in three seconds it gets dizzy.
- **Rub** it with your finger to polish it. Shine controls how saturated and glittery
  it looks, and how many sparkles drift off it.
- **Snack** throws something odd at it (moon cheese, comet noodle, a single sock…).
  It spits out the snack it hates, cheers for its favourite, and gets glittery
  hiccups if you overfeed it.
- **Bubbles** starts a short game: pop the bubbles before they float away. Golden
  bubbles also add shine.
- **Polish** spins it through a sparkle shower (12 s cooldown).
- **Nap** dims the island and refills Pep. It wakes up on its own when it is rested
  and tells you what it dreamt.

There are four stats: Tummy, Giggles, Shine and Pep. They drain in real time, and
the time you are away counts too, capped at 12 hours. When the pet's mood drops,
its colour cools, its eyelids droop and a small rain cloud follows it around.
Nothing dies. Care actions give XP. Levels unlock orbiting moonlets (from Lv 2), a
party hat (Lv 3) and a halo (Lv 5).

Sound is synthesised with WebAudio and has a toggle in the top-right corner. State
is saved in `localStorage`.

## Rendering notes

- The body, egg and limbs share one `ShaderMaterial` with vertex wobble, a poke dent
  where you tap, a fresnel-driven cosine-palette iridescence, and per-cell
  procedural glitter that twinkles. Desaturation is driven by the Shine stat.
- The googly eyes are damped springs that overshoot and bounce off the rim of the eye.
- All sparkles, including bursts, rain and the ambient star field, are drawn with a
  single additive point shader that draws a four-point star procedurally, so the
  page needs no textures.

To run it locally, serve the folder (`python3 -m http.server`) and open
`index.html`. It is written as an Artifact page, so it has no `<html>`/`<head>`
wrapper. Browsers handle that fine.
