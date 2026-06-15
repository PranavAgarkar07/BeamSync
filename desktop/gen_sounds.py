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

# 1. Hover: Sharp, short, high-tech "Tick"
# High frequency blip with fast decay
hover_samples = []
for t in range(int(SR * 0.05)): # 0.05 seconds
    env = 1.0 - (t / (SR * 0.05)) # Linear decay
    # Frequency modulation for "chirp"
    val = math.sin(2 * math.pi * (2000 - t/10) * t / SR) * env
    hover_samples.append(val * 0.3)

save_wav("build/bin/sounds/hover.wav", hover_samples)

# Separate click sound - softer UI pop
click_samples = []
for t in range(int(SR * 0.05)):
    progress = t / (SR * 0.05)

    env = math.exp(-6 * progress)

    fundamental = math.sin(2 * math.pi * 1200 * t / SR)
    harmonic = 0.4 * math.sin(2 * math.pi * 2400 * t / SR)

    val = (fundamental + harmonic) * env

    click_samples.append(val * 0.25)

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
