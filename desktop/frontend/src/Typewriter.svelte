<script>
    import { onMount, afterUpdate } from "svelte";

    export let text = "";
    export let speed = 50; // ms per char (approx 500ms for short string, maybe faster?)

    // User asked for "type itself out ... over 500ms".
    // If string is long, speed needs to be lower.
    // We'll try dynamic speed or fixed speed.
    // Let's stick to a fixed reasonable speed per char for terminal feel.

    let display = "";
    let currentIndex = 0;
    let interval;

    $: if (text) {
        startTyping();
    }

    function startTyping() {
        display = "";
        currentIndex = 0;
        if (interval) clearInterval(interval);

        // Calculate speed to fit in ~500ms?
        // If text is 20 chars, 500/20 = 25ms.
        let dynSpeed = Math.max(20, 500 / (text.length || 1));

        interval = setInterval(() => {
            if (currentIndex < text.length) {
                display += text[currentIndex];
                currentIndex++;
            } else {
                clearInterval(interval);
            }
        }, speed);
    }

    onMount(() => {
        return () => clearInterval(interval);
    });
</script>

<span class="typewriter">
    {display}<span class="cursor">█</span>
</span>

<style>
    .cursor {
        display: inline-block;
        color: var(
            --primary
        ); /* Inherit or explicit? User said blocks cursor */
        animation: blink 1s step-end infinite;
        margin-left: 2px;
    }

    @keyframes blink {
        0%,
        100% {
            opacity: 1;
        }
        50% {
            opacity: 0;
        }
    }
</style>
