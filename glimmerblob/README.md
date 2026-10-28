# Glimmerblob

A mobile-first three.js pocket pet: a Tamagotchi-like creature, but quirkier and
sparklier. Everything is in one self-contained `index.html`. It loads three.js r128
from cdnjs and the fonts from Google Fonts.

## Playing

- **Hatch** the egg by tapping it three times. Every egg gets a random name (tap the
  name to rename it), a colour, a secret favourite snack and one snack it hates.
  A welcome present drops a few seconds after it hatches.
- **Boop** it with a tap. Seven quick boops in three seconds make it dizzy.
- **Rub** it to polish it. Shine controls how colourful and glittery it looks.
  Rub really fast and it has a **tickle attack**.
- **Hold and fling**: press and hold for a moment to pick it up, then throw it. It
  bounces off the island and the walls, spins, and gets dizzy after a big drop.
  You can catch it again mid-air. With wings (Lv 4) it floats down more slowly.
- **Snack** throws something odd at it (moon cheese, comet noodle, a single sock…).
  It spits out the snack it hates, cheers for its favourite, and gets glittery
  hiccups if you overfeed it.
- **Polish** spins it through a sparkle shower (12 s cooldown).
- **Nap** dims the island and refills Pep. It wakes up on its own and tells you
  what it dreamt.

### Play (mini-games)

- **Bubble Pop**: tap bubbles before they float away. Quick pops chain into combos,
  and golden bubbles add shine.
- **Star Catch**: drag left and right (or use the arrow keys) to catch falling stars
  for 30 seconds. Rainbow comets are worth 5. Gloom rocks bonk you, stun you and
  break your combo. Every 6 catches in a row raises the multiplier, up to x5.
- **Disco Dance**: a 116 BPM rhythm game with a synthesised beat, a disco ball and
  party lights. Tap (or press Space) when each ring lands on the circle. Perfect
  is ±70 ms and good is ±160 ms. You get a rank from S to D.

High scores are kept per game. Scoring 45 or more in Star Catch, or an A or S in
Disco Dance, drops a present.

### Shooting stars, presents and the closet

Every minute or so a shooting star crosses the sky. Tap it to make a wish: a
present, a sparkle storm, or a giggle fit. Presents fall onto the island and
wiggle until you tap them. Each one unlocks a new outfit: a tiny crown, top hat,
propeller cap (it spins when the blob flies), bunny ears, flower crown, big bow,
star shades or a fancy mustache. It wears one head item and one face item at a
time, and you change them in the closet (hanger button). You also get presents
on every even level and once a day.

### Stats and growth

There are four stats: Tummy, Giggles, Shine and Pep. They drain in real time, and
the time you are away counts too, capped at 12 hours. When its mood drops, its
colour cools, its eyelids droop and a small rain cloud follows it. Nothing dies.
Care and games give XP. Levels unlock orbiting moonlets (Lv 2+), a party hat
(Lv 3), fairy wings (Lv 4) and a halo (Lv 5).

Sound is synthesised with WebAudio and has a toggle in the top-right corner. State
is saved in `localStorage`.

## Rendering notes

- The body, egg and limbs share one `ShaderMaterial` with vertex wobble, a poke dent
  where you tap, a fresnel-driven cosine-palette iridescence, and per-cell
  procedural glitter that twinkles. Desaturation is driven by the Shine stat.
- The googly eyes are damped springs that overshoot and bounce off the rim of the eye.
- Flinging is a small hand-rolled physics step: velocity is measured while you drag,
  then gravity, restitution and spin take over. The spin pivots around the body's
  centre, not its feet.
- The Disco Dance track is scheduled on the WebAudio clock a little ahead of the
  beat. Notes are judged against `performance.now()`, so the timing does not
  depend on the frame rate.
- All sparkles, including bursts, rain and the ambient star field, are drawn with a
  single additive point shader that draws a four-point star procedurally, so the
  page needs no textures.

To run it locally, serve the folder (`python3 -m http.server`) and open
`index.html`. It is written as an Artifact page, so it has no `<html>`/`<head>`
wrapper. Browsers handle that fine.
