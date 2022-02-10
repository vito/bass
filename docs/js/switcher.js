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
  var link = document.createElement('link');
  link.rel = "stylesheet";
  link.type = "text/css";
  link.href = "css/base16/base16-"+style+".css";
  link.media = "all";

  // only swap the element once the CSS has loaded to prevent flickering
  link.onload = function() {
    // switcheroo
    var prevLink = document.getElementById(linkId)
    if (prevLink) {
      prevLink.remove();
    }

    link.id = linkId;
  };

  document.head.appendChild(link);

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
    chooser.appendChild(reset);
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
  // "material-darker", // base03 too low contrast
  "mocha",
  "monokai",
  // "nord", // base03 too low contrast
  "ocean",
  "oceanicnext",
  "outrun-dark",
  "rose-pine",
  "rose-pine-moon",
  "snazzy",
  "tender",
  "tokyo-night-dark",
  "tokyo-night-terminal-dark",
  "tomorrow-night",
  "tomorrow-night-eighties",
  "twilight",
  "woodland",
]


var defaultStyle = curatedStyles[Math.floor(Math.random()*curatedStyles.length)]

// preload all curated styles to prevent flickering
curatedStyles.forEach(function(style) {
  var link = link = document.createElement('link');
  link.rel = "alternate stylesheet";
  link.title = style;
  link.type = "text/css";
  link.href = "css/base16/base16-"+style+".css";
  link.media = "all";
  document.head.appendChild(link);
});

setStyleOrDefault(defaultStyle);

window.onload = function() {
  // call again to update switcher selection
  setStyleOrDefault(defaultStyle);

  document.querySelectorAll(".stderr pre").forEach(function(item) {
    item.scrollTop = item.scrollHeight;
  });
}
