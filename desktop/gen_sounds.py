import wave
import math
import struct
import random
import os

def save_wav(filename, samples, sample_rate=44100):
    with wave.open(filename, 'w') as w:
        w.setnchannels(1)
        w.setsampwidth(2)
        w.setframerate(sample_rate)
        w.writeframes(b''.join(struct.pack('<h', int(max(min(s, 1.0), -1.0) * 32767.0)) for s in samples))
    print(f"Generated {filename}")

SR = 44100

# 1. Hover: Soft shimmer
hover_samples = []

duration = 0.06

for t in range(int(SR * duration)):
    progress = t / (SR * duration)

    env = math.exp(-4 * progress)

    tone = math.sin(2 * math.pi * 1400 * t / SR)

    noise = (random.random() * 2 - 1) * 0.15

    val = (tone * 0.8 + noise * 0.2) * env

    hover_samples.append(val * 0.15)

save_wav("build/bin/sounds/hover.wav", hover_samples)

# Click: soft tactile button press
click_samples = []

duration = 0.05

for t in range(int(SR * duration)):
    progress = t / (SR * duration)

    env = math.exp(-8 * progress)

    body = math.sin(2 * math.pi * 600 * t / SR)

    noise = (random.random() * 2 - 1) * 0.4

    val = (body * 0.7 + noise * 0.3) * env

    click_samples.append(val * 0.3)

save_wav("build/bin/sounds/click.wav", click_samples)

# 2. Connect: Warm acknowledgment chime
connect_samples = []
duration = 0.22

for t in range(int(SR * duration)):
    progress = t / (SR * duration)

    env = math.exp(-3 * progress)

    base = math.sin(2 * math.pi * 880 * t / SR)
    harmonic = 0.5 * math.sin(2 * math.pi * 1320 * t / SR)

    val = (base + harmonic) * env

    connect_samples.append(val * 0.25)

save_wav("build/bin/sounds/connect.wav", connect_samples)

# 3. Startup: Modern ascending startup chime
startup_samples = []

notes = [523.25, 659.25, 783.99]  # C5 E5 G5
note_duration = 0.22
total_duration = 0.9

for t in range(int(SR * total_duration)):
    current_time = t / SR

    if current_time < note_duration:
        freq = notes[0]
    elif current_time < note_duration * 2:
        freq = notes[1]
    else:
        freq = notes[2]

    progress = current_time / total_duration

    env = math.exp(-2.5 * progress)

    base = math.sin(2 * math.pi * freq * t / SR)
    harmonic = 0.2 * math.sin(2 * math.pi * freq * 2 * t / SR)

    val = (base + harmonic) * env

    startup_samples.append(val * 0.18)

save_wav("build/bin/sounds/startup.wav", startup_samples)

# 4. Success: Modern completion chime
success_samples = []

notes = [523.25, 659.25, 783.99]
note_duration = 0.18
total_duration = 0.8

for t in range(int(SR * total_duration)):
    current_time = t / SR

    if current_time < note_duration:
        freq = notes[0]
    elif current_time < note_duration * 2:
        freq = notes[1]
    else:
        freq = notes[2]

    progress = current_time / total_duration

    env = math.exp(-2.5 * progress)

    base = math.sin(2 * math.pi * freq * t / SR)
    harmonic = 0.35 * math.sin(2 * math.pi * freq * 2 * t / SR)

    val = (base + harmonic) * env

    success_samples.append(val * 0.25)

save_wav("build/bin/sounds/transfer_complete.wav", success_samples)

print("Cyberpunk Audio Assets Generated Successfully.")
