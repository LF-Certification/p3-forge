document$.subscribe(function() {
	if (!navigator.clipboard) return;
	document.querySelectorAll(".md-typeset code.copy").forEach(function(el) {
		if (el.closest("pre")) return;
		if (el.hasAttribute("data-inline-copy")) return;
		el.setAttribute("data-inline-copy", "");
		el.setAttribute("tabindex", "0");
		el.setAttribute("role", "button");
		el.classList.add("inline-code-copy");
		function copy() {
			navigator.clipboard.writeText(el.innerText).then(function() {
				el.classList.add("inline-code-copied");
				setTimeout(function() { el.classList.remove("inline-code-copied"); }, 1000);
			});
		}
		el.addEventListener("click", copy);
		el.addEventListener("keydown", function(e) {
			if (e.key === "Enter" || e.key === " ") {
				e.preventDefault();
				copy();
			}
		});
	});
});