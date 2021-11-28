const styleKey = "theme";
const linkId = "theme";

const controlsId = "choosetheme"
const switcherId = "styleswitcher";
const resetId = "resetstyle";

function storeStyle(style) {
  window.localStorage.setItem(styleKey, style);
}

function loadStyle() {
  var style = window.localStorage.getItem(styleKey);

  if (style !== null && window.goatcounter !== undefined && window.goatcounter.count !== undefined) {
    window.goatcounter.count({
      path:  `style/${style}`,
      title: "Loaded user-configured style",
      event: true,
    })
  }

  return style
}

function setActiveStyle(style) {
  var link = document.getElementById(linkId);
  if (link) {
    link.href = "css/base16/base16-"+style+".css";
  } else {
    link = document.createElement('link');
    link.id = linkId;
    link.rel = "stylesheet";
    link.type = "text/css";
    link.href = "css/base16/base16-"+style+".css";
    link.media = "all";
    document.head.appendChild(link);
  }

  var switcher = document.getElementById(switcherId);
  if (switcher) {
    // might not be loaded yet; this function is called twice, once super early
    // to prevent flickering, and again once all the dom is loaded up
    switcher.value = style;
  }

  resetReset();
}

function resetReset() {
  var style = loadStyle();
  var reset = document.getElementById(resetId);
  if (!style) {
    if (reset) {
      // no style selected; remove reset element
      reset.remove();
    }

    return
  }

  if (reset) {
    // no style and no reset; done
    return
  }

  // has style but no reset element
  reset = document.createElement("a");
  reset.id = resetId;
  reset.onclick = resetStyle;
  reset.href = 'javascript:void(0)';
  reset.text = "reset";
  reset.className = "reset";

  var chooser = document.getElementById(controlsId);
  if (chooser) {
    chooser.prepend(reset);
  }
}

function setStyleOrDefault(def) {
  setActiveStyle(loadStyle() || def);
}

function switchStyle(event) {
  var style = event.target.value;
  storeStyle(style);
  setActiveStyle(style);
}

function resetStyle() {
  window.localStorage.removeItem(styleKey);
  setActiveStyle(defaultStyle);
}

var curatedStyles = [
  "chalk",
  "classic-dark",
  "darkmoss",
  "decaf",
  "default-dark",
  "dracula",
  "eighties",
  "equilibrium-dark",
  "equilibrium-gray-dark",
  "espresso",
  "framer",
  "gruvbox-dark-medium",
  "hardcore",
  "horizon-dark",
  "horizon-terminal-dark",
  "ir-black",
  "materia",
  "material",
  "material-darker",
  "mocha",
  "monokai",
  "nord",
  "ocean",
  "oceanicnext",
  "outrun-dark",
  "rose-pine",
  "rose-pine-moon",
  "snazzy",
  "tender",
  "tokyo-night-dark",
  "tokyo-night-terminal",
  "tomorrow-night",
  "tomorrow-night-eighties",
  "twilight",
  "woodland",
]

var defaultStyle = curatedStyles[Math.floor(Math.random()*curatedStyles.length)]

setStyleOrDefault(defaultStyle);

window.onload = function() {
  // call again to update switcher selection
  setStyleOrDefault(defaultStyle);
}
