(() => {
  const intervals = new WeakMap();

  function render(element) {
    element.innerHTML = `
      <div style="display:flex;flex-direction:column;gap:16px;height:100%;background:#0f172a;color:#e2e8f0;padding:24px;box-sizing:border-box;font-family:system-ui,sans-serif;">
        <div style="font-size:14px;opacity:0.75;">External JS module mounted into WebElementView</div>
        <div style="background:#1e293b;border:1px solid #334155;border-radius:16px;padding:18px;">
          <div style="font-size:28px;font-weight:700;margin-bottom:8px;">Direct Callback Bridge</div>
          <div style="line-height:1.6;max-width:560px;">
            JS owns this subtree. Kotlin updates the message through <code>updateDemo</code>.
            JS calls Kotlin back through the <code>onTick</code> and <code>onButtonPressed</code> callbacks.
          </div>
        </div>
        <div style="background:#132033;border:1px solid #233652;border-radius:14px;padding:16px;">
          <div style="font-weight:700;margin-bottom:6px;">Message from Kotlin</div>
          <div id="bridge-message" style="font-size:18px;">Waiting for Kotlin update...</div>
        </div>
        <div style="display:flex;gap:12px;align-items:center;">
          <button id="bridge-button" style="border:0;border-radius:999px;padding:10px 16px;background:#38bdf8;color:#082f49;font-weight:700;cursor:pointer;">
            Send Direct Callback
          </button>
          <span style="opacity:0.7;">This button calls Kotlin directly.</span>
        </div>
      </div>
    `;
  }

  globalThis.AladinBridge = {
    mountDemo(element, onTick, onButtonPressed) {
      render(element);

      const button = element.querySelector("#bridge-button");
      if (button) {
        button.onclick = () => onButtonPressed();
      }

      let count = 0;
      const intervalId = setInterval(() => {
        count += 1;
        onTick(count);
      }, 1000);

      intervals.set(element, intervalId);
    },

    updateDemo(element, message) {
      const messageNode = element.querySelector("#bridge-message");
      if (messageNode) {
        messageNode.textContent = message;
      }
    },

    disposeDemo(element) {
      const intervalId = intervals.get(element);
      if (intervalId != null) {
        clearInterval(intervalId);
        intervals.delete(element);
      }

      const button = element.querySelector("#bridge-button");
      if (button) {
        button.onclick = null;
      }
    }
  };
})();
