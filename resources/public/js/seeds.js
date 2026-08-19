
$(document).ready(function () {
    showLoader();
    
	$.get(appUrl("/api/seeds"), function (seeds) {
	        hideLoader();
			$("#seeds_loading").hide();
	        var hasActive = false;
			if (seeds.length == 0) {
				$("#seeds_empty").show();
			} else {
				$("#seeds_table_container").show();
			}
	        seeds.forEach(function (seed) {
	    		appendSeedDetails(seed, "[data-agent=seed_details]");
	    		if (!seed.IsComplete) {
	    			hasActive = true;
	    		}
	    	});
    		if (hasActive) {
    			activateRefreshTimer();
    		}
	    }, "json").fail(function() {
			hideLoader();
			$("#seeds_loading").hide();
			$("#seeds_unavailable").show();
		});
});	
