// Open external links in new tab (for iframe embedding)
// Uses document$ observable for instant loading compatibility
document$.subscribe(function() {
  document.querySelectorAll('a[href^="http"]').forEach(function(link) {
    try {
      var linkHost = new URL(link.href).hostname;
      var currentHost = window.location.hostname;
      if (linkHost !== currentHost) {
        link.setAttribute('target', '_blank');
        link.setAttribute('rel', 'noopener noreferrer');
      }
    } catch (e) {
      // Invalid URL, skip
    }
  });
});
