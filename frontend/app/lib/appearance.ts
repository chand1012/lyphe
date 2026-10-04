export function applyAppearance() {
  document.documentElement.dataset.contentFont = localStorage.getItem("lyphe-font") === "geist" ? "geist" : "lora";
  document.documentElement.classList.toggle("compact", localStorage.getItem("lyphe-compact") === "true");
}
