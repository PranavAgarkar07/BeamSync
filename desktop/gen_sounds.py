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
# Alias for click/blip
save_wav("build/bin/sounds/click.wav", hover_samples)


# 2. Connect: "Data Uplink" (Two-tone digital handshake)
# 800Hz -> 1600Hz
connect_samples = []
duration = 0.2
swap_point = int(SR * 0.1)
for t in range(int(SR * duration)):
    freq = 800 if t < swap_point else 1600
    val = math.sin(2 * math.pi * freq * t / SR)
    # Add slight distortion (square-ish)
    val = 1.0 if val > 0 else -1.0
    connect_samples.append(val * 0.15)

save_wav("build/bin/sounds/connect.wav", connect_samples)


# 3. Startup: "System Power Up" (Rising sweep with vibrato)
startup_samples = []
duration = 1.5
for t in range(int(SR * duration)):
    progress = t / (SR * duration)
    # Pitch rises from 100Hz to 800Hz
    base_freq = 100 + (700 * (progress**2)) # Exponential rise
    # Add "Cyberpunk" Vibrato
    vibrato = math.sin(2 * math.pi * 30 * t / SR) * 20 
    
    phase = 2 * math.pi * (base_freq + vibrato) * t / SR
    
    # Sawtooth-like synthesis (additive)
    val = (math.sin(phase) + 0.5 * math.sin(phase * 2)) / 1.5
    
    # Envelope
    env = 1.0
    if progress > 0.8: # Fade out at end
        env = 1.0 - ((progress - 0.8) / 0.2)
    elif progress < 0.1: # Fade in
        env = progress / 0.1
        
    startup_samples.append(val * env * 0.4)

save_wav("build/bin/sounds/startup.wav", startup_samples)


# 4. Success: "Task Complete" (Major Triad Arpeggio)
# C5 (523.25), E5 (659.25), G5 (783.99)
notes = [523.25, 659.25, 783.99, 1046.50] # C E G C(high)
note_len = 0.08
total_len = note_len * len(notes) + 0.5 # decay
success_samples = []

current_note_idx = 0
time_in_note = 0

for t in range(int(SR * total_len)):
    if current_note_idx < len(notes):
        freq = notes[current_note_idx]
        val = math.sin(2 * math.pi * freq * t / SR)
        # Square wave for "retro" feel
        val = 0.5 if val > 0 else -0.5
        
        success_samples.append(val * 0.2)
        
        time_in_note += 1
        if time_in_note > SR * note_len:
            time_in_note = 0
            current_note_idx += 1
    else:
        # Reverb/Decay tail
        val = (math.sin(2 * math.pi * notes[-1] * t / SR) + math.sin(2 * math.pi * notes[0] * t / SR)) * 0.5
        # Fade out
        remaining = 1.0 - ((t - (SR * note_len * len(notes))) / (SR * 0.5))
        if remaining < 0: remaining = 0
        success_samples.append(val * 0.2 * remaining)

save_wav("build/bin/sounds/transfer_complete.wav", success_samples)

print("Cyberpunk Audio Assets Generated Successfully.")
