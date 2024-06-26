// TODO(peter):
//
// - interactions
//   - mouse wheel: horizontal zoom
//   - click/drag: horizontal pan

"use strict";

// The heights of each level. The first few levels are given smaller
// heights to account for the increasing target file size.
//
// TODO(peter): Use the TargetFileSizes specified in the OPTIONS file.
let levelHeights = [16, 16, 16, 16, 32, 64, 128];
const offsetStart = 24;
let levelOffsets = generateLevelOffsets();
const lineStart = 105;
const sublevelHeight = 16;
let levelWidth = 0;

{
    // Create the base DOM elements.
    let c = d3
        .select("body")
        .append("div")
        .attr("id", "container");
    let h = c.append("div").attr("id", "header");
    h
        .append("div")
        .attr("id", "index-container")
        .append("input")
        .attr("type", "text")
        .attr("id", "index")
        .attr("autocomplete", "off");
    let checkboxContainer = h
        .append("div")
        .attr("id", "checkbox-container");
    checkboxContainer.append("input")
        .attr("type", "checkbox")
        .attr("id", "flatten-sublevels")
        .on("change", () => {version.onCheckboxChange(d3.event.target.checked)});
    checkboxContainer.append("label")
        .attr("for", "flatten-sublevels")
        .text("Show sublevels");
    h.append("svg").attr("id", "slider");
    c.append("svg").attr("id", "vis");
}

let vis = d3.select("#vis");

function renderHelp() {
    vis
        .append("text")
        .attr("class", "help")
        .attr("x", 10)
        .attr("y", levelOffsets[6] + 30)
        .text(
            "(space: start/stop, left-arrow[+shift]: step-back, right-arrow[+shift]: step-forward)"
        );
}

function renderReason() {
    return vis
        .append("text")
        .attr("class", "reason")
        .attr("x", 10)
        .attr("y", 16);
}

let reason = renderReason();

let index = d3.select("#index");

// Pretty formatting of a number in human readable units.
function humanize(s) {
    const iecSuffixes = [" B", " KB", " MB", " GB", " TB", " PB", " EB"];
    if (s < 10) {
        return "" + s;
    }
    let e = Math.floor(Math.log(s) / Math.log(1024));
    let suffix = iecSuffixes[Math.floor(e)];
    let val = Math.floor(s / Math.pow(1024, e) * 10 + 0.5) / 10;
    return val.toFixed(val < 10 ? 1 : 0) + suffix;
}

function generateLevelOffsets() {
    return levelHeights.map((v, i) =>
        levelHeights.slice(0, i + 1).reduce((sum, elem) => sum + elem, offsetStart)
    );
}

function styleWidth(e) {
    let width = +e.style("width").slice(0, -2);
    return Math.round(Number(width));
}

function styleHeight(e) {
    let height = +e.style("height").slice(0, -2);
    return Math.round(Number(height));
}

let sliderX, sliderHandle;
let offsetSliderX;

// The version object holds the current LSM state.
let version = {
    levels: [[], [], [], [], [], [], []],
    sublevels: [],
    numSublevels: 0,
    showSublevels: false,
    // Generated after every change using setLevelsInfo().
    levelsInfo: [],
    // The version edit index.
    index: -1,

    init: function() {
        for (let edit of data.Edits) {
            if (edit.Sublevels === null || edit.Sublevels === undefined) {
                continue;
            }
            for (let [file, sublevel] of Object.entries(edit.Sublevels)) {
                if (sublevel >= this.numSublevels) {
                    this.numSublevels = sublevel + 1;
                }
            }
        }
        for (let i = 0; i < this.numSublevels; i++) {
            this.sublevels.push([]);
        }
        d3.select("#checkbox-container label")
            .text("Show sublevels (" + this.numSublevels.toString() + ")");
        this.setHeights();
        this.setLevelsInfo();
        renderHelp();
    },

    setHeights: function() {
        // Update the height of level 0 to account for the number of sublevels,
        // if there are any.
        if (this.numSublevels > 0 && this.showSublevels === true) {
            levelHeights[0] = sublevelHeight * this.numSublevels;
        } else {
            levelHeights[0] = sublevelHeight;
        }
        levelOffsets = generateLevelOffsets();
        vis.style("height", levelOffsets[6] + 100);
    },

    onCheckboxChange: function(value) {
        this.showSublevels = value;
        vis.selectAll("*")
            .remove();
        reason = renderReason();
        this.setHeights();
        this.setLevelsInfo();
        renderHelp();

        this.render(true);
        this.updateSize();
    },

    // Set the version edit index. This steps either forward or
    // backward through the version edits, applying or unapplying each
    // edit.
    set: function(index) {
        let prevIndex = this.index;
        if (index < 0) {
            index = 0;
        } else if (index >= data.Edits.length) {
            index = data.Edits.length - 1;
        }
        if (index == this.index) {
            return;
        }

        // If the current edit index is less than the target index,
        // step forward applying edits.
        for (; this.index < index; this.index++) {
            let edit = data.Edits[this.index + 1];
            for (let level in edit.Deleted) {
                this.remove(level, edit.Deleted[level]);
            }
            for (let level in edit.Added) {
                this.add(level, edit.Added[level]);
            }
        }

        // If the current edit index is greater than the target index,
        // step backward unapplying edits.
        for (; this.index > index; this.index--) {
            let edit = data.Edits[this.index];
            for (let level in edit.Added) {
                this.remove(level, edit.Added[level]);
            }
            for (let level in edit.Deleted) {
                this.add(level, edit.Deleted[level]);
            }
        }

        // Build the sublevels from this.levels[0]. They need to be rebuilt from
        // scratch each time there's a change to L0.
        this.sublevels = [];
        while(this.sublevels.length < this.numSublevels) {
            this.sublevels.push([]);
        }
        for (let file of this.levels[0]) {
            let sublevel = null;
            for (let i = index; i >= 0 && (sublevel === null || sublevel === undefined); i--) {
                if (data.Edits[i].Sublevels == null || data.Edits[i].Sublevels == undefined) {
                  continue;
                }
                sublevel = data.Edits[i].Sublevels[file];
            }
            this.sublevels[sublevel].push(file);
        }

        // Sort the levels.
        for (let i in this.levels) {
            if (i == 0) {
                for (let j in this.sublevels) {
                    this.sublevels[j].sort(function(a, b) {
                        let fa = data.Files[a];
                        let fb = data.Files[b];
                        if (fa.Smallest < fb.Smallest) {
                            return -1;
                        }
                        if (fa.Smallest > fb.Smallest) {
                            return +1;
                        }
                        return 0;
                    });
                }
                this.levels[i].sort(function(a, b) {
                    let fa = data.Files[a];
                    let fb = data.Files[b];
                    if (fa.LargestSeqNum < fb.LargestSeqNum) {
                        return -1;
                    }
                    if (fa.LargestSeqNum > fb.LargestSeqNum) {
                        return +1;
                    }
                    if (fa.SmallestSeqNum < fb.SmallestSeqNum) {
                        return -1;
                    }
                    if (fa.SmallestSeqNum > fb.SmallestSeqNum) {
                        return +1;
                    }
                    return a < b;
                });
            } else {
                this.levels[i].sort(function(a, b) {
                    let fa = data.Files[a];
                    let fb = data.Files[b];
                    if (fa.Smallest < fb.Smallest) {
                        return -1;
                    }
                    if (fa.Smallest > fb.Smallest) {
                        return +1;
                    }
                    return 0;
                });
            }
        }

        this.updateLevelsInfo();
        this.render(prevIndex === -1);
    },

    // Add the specified sstables to the specifed level.
    add: function(level, fileNums) {
        for (let i = 0; i < fileNums.length; i++) {
            this.levels[level].push(fileNums[i]);
        }
    },

    // Remove the specified sstables from the specifed level.
    remove: function(level, fileNums) {
        let l = this.levels[level];
        for (let i = 0; i < l.length; i++) {
            if (fileNums.indexOf(l[i]) != -1) {
                l[i] = l[l.length - 1];
                l.pop();
                i--;
            }
        }
    },

    // Return the size of the sstables in a level.
    size: function(level, sublevel) {
        if (level == 0 && sublevel !== null && sublevel !== undefined) {
            return this.sublevels[sublevel].reduce(
                (sum, elem) => sum + data.Files[elem].Size,
                0
            );
        }
        return (this.levels[level] || []).reduce(
            (sum, elem) => sum + data.Files[elem].Size,
            0
        );
    },

    // Returns the height to use for an sstable.
    height: function(fileNum) {
        let meta = data.Files[fileNum];
        return Math.ceil((meta.Size + 1024.0 * 1024.0 - 1) / (1024.0 * 1024.0));
    },

    scale: function(level) {
        return levelWidth < this.levelsInfo[level].files.length
            ? levelWidth / this.levelsInfo[level].files.length
            : 1;
    },

    // Return a summary of the count and size of the specified sstables.
    summarize: function(level, fileNums) {
        let count = 0;
        let size = 0;
        for (let fileNum of fileNums) {
            count++;
            size += data.Files[fileNum].Size;
        }
        return count + " @ " + "L" + level + " (" + humanize(size) + ")";
    },

    // Return a textual description of a version edit.
    describe: function(edit) {
        let s = edit.Reason;

        if (edit.Deleted) {
            let sep = " ";
            for (let i = 0; i < 7; i++) {
                if (edit.Deleted[i]) {
                    s += sep + this.summarize(i, edit.Deleted[i]);
                    sep = " + ";
                }
            }
        }

        if (edit.Added) {
            let sep = " => ";
            for (let i = 0; i < 7; i++) {
                if (edit.Added[i]) {
                    s += sep + this.summarize(i, edit.Added[i]);
                    sep = " + ";
                }
            }
        }

        return s;
    },

    setLevelsInfo: function() {
        let sublevelCount = this.numSublevels;
        let levelsInfo = [];
        let levelsStart = 1;
        if (this.showSublevels === true) {
            levelsInfo = this.sublevels.map((files, sublevel) => ({
                files: files,
                levelString: "L0." + sublevel.toString(),
                levelDisplayString: (sublevel === this.numSublevels - 1 ?
                    "L0." : "&nbsp;&nbsp;&nbsp;&nbsp;.") + sublevel.toString(),
                levelClass: "L0-" + sublevel.toString(),
                level: 0,
                offset: offsetStart + (sublevelHeight * (sublevelCount - sublevel)),
                height: sublevelHeight,
                size: humanize(this.size(0, sublevel)),
            }));
            if (levelsInfo.length === 0) {
                levelsStart = 0;
            }
            levelsInfo.reverse();
        } else {
            levelsStart = 0;
        }

        levelsInfo = levelsInfo.concat(this.levels.slice(levelsStart).map((files, level) => ({
            files: files,
            levelString: "L" + (level+levelsStart).toString(),
            levelDisplayString: "L" + (level+levelsStart).toString(),
            levelClass: "L" + (level+levelsStart).toString(),
            level: level,
            offset: levelOffsets[level+levelsStart],
            height: levelHeights[level+levelsStart],
            size: humanize(this.size(level+levelsStart)),
        })));
        this.levelsInfo = levelsInfo;
    },

    updateLevelsInfo: function() {
        let levelsStart = 1;
        if (this.showSublevels === true) {
            this.sublevels.forEach((files, sublevel) => {
                this.levelsInfo[this.numSublevels - (sublevel + 1)].files = files;
                this.levelsInfo[this.numSublevels - (sublevel + 1)].size = humanize(this.size(0, sublevel));
            });
            if (this.numSublevels === 0) {
                levelsStart = 0;
            }
        } else {
            levelsStart = 0;
        }

        this.levels.slice(levelsStart).forEach((files, level) => {
            let sublevelOffset = this.showSublevels === true ? this.numSublevels : 0;
            this.levelsInfo[sublevelOffset + level].files = files;
            this.levelsInfo[sublevelOffset + level].size = humanize(this.size(levelsStart + level));
        });
    },

    render: function(redraw) {
        let version = this;

        vis.interrupt();

        // Render the edit info.
        let info = "[" + this.describe(data.Edits[this.index]) + "]";
        reason.text(info);

        // Render the text for each level: sstable count and size.
        vis
            .selectAll("text.levels")
            .data(this.levelsInfo)
            .enter()
            .append("text")
            .attr("class", "levels")
            .attr("x", 10)
            .attr("y", d => d.offset)
            .html(d => d.levelDisplayString);
        vis
            .selectAll("text.counts")
            .data(this.levelsInfo)
            .text((d, i) => d.files.length)
            .enter()
            .append("text")
            .attr("class", "counts")
            .attr("text-anchor", "end")
            .attr("x", 55)
            .attr("y", d => d.offset)
            .text(d => d.files.length);
        vis
            .selectAll("text.sizes")
            .data(this.levelsInfo)
            .text((d, i) => d.size)
            .enter()
            .append("text")
            .attr("class", "sizes")
            .attr("text-anchor", "end")
            .attr("x", 100)
            .attr("y", (d, i) => d.offset)
            .text(d => d.size);

        // Render each of the levels. Each level is composed of an
        // outer group which provides a clipping recentangle, an inner
        // group defining the coordinate system, an overlap rectangle
        // to capture mouse events, an indicator rectangle used to
        // display sstable overlaps, and the per-sstable rectangles.
        for (let i in this.levelsInfo) {
            let g, clipG;
            if (redraw === false) {
                g = vis
                    .selectAll("g.clip" + this.levelsInfo[i].levelClass)
                    .select("g")
                    .data([i]);
                clipG = g
                    .enter()
                    .append("g")
                    .attr("class", "clipRect clip" + this.levelsInfo[i].levelClass)
                    .attr("clip-path", "url(#" + this.levelsInfo[i].levelClass + ")");
            } else {
                clipG = vis
                    .append("g")
                    .attr("class", "clipRect clip" + this.levelsInfo[i].levelClass)
                    .attr("clip-path", "url(#" + this.levelsInfo[i].levelClass + ")")
                    .data([i]);
                g = clipG
                    .append("g");
            }
            clipG
                .append("g")
                .attr(
                    "transform",
                    "translate(" +
                        lineStart +
                        "," +
                        this.levelsInfo[i].offset +
                        ") scale(1,-1)"
                );
            clipG.append("rect").attr("class", "indicator");

            // Define the overlap rectangle for capturing mouse events.
            clipG
                .append("rect")
                .attr("x", lineStart)
                .attr("y", this.levelsInfo[i].offset - this.levelsInfo[i].height)
                .attr("width", levelWidth)
                .attr("height", this.levelsInfo[i].height)
                .attr("opacity", 0)
                .attr("pointer-events", "all")
                .on("mousemove", i => version.onMouseMove(i))
                .on("mouseout", function() {
                    reason.text(info);
                    vis.selectAll("rect.indicator").attr("fill", "none");
                });

            // Scale each level to fit within the display.
            let s = this.scale(i);
            g.attr(
                "transform",
                "translate(" +
                    lineStart +
                    "," +
                    this.levelsInfo[i].offset +
                    ") scale(" +
                    s +
                    "," +
                    -(1 / s) +
                    ")"
            );

            // Render the sstables for the level.
            let level = g.selectAll("rect." + this.levelsInfo[i].levelClass).data(this.levelsInfo[i].files);
            level.attr("fill", "#555").attr("x", (fileNum, i) => i);
            level
                .enter()
                .append("rect")
                .attr("class", this.levelsInfo[i].levelClass + " sstable")
                .attr("id", fileNum => fileNum)
                .attr("fill", "red")
                .attr("x", (fileNum, i) => i)
                .attr("y", 0)
                .attr("width", 1)
                .attr("height", fileNum => version.height(fileNum));
            level.exit().remove();
        }

        sliderHandle.attr("cx", sliderX(version.index));
        index.node().value = version.index + data.StartEdit;
    },

    onMouseMove: function(i) {
        i = Number(i);
        if (Number.isNaN(i) || i >= this.levelsInfo.length || this.levelsInfo[i].files.length === 0) {
            return;
        }

        // The mouse coordinates are relative to the
        // SVG element. Adjust to be relative to the
        // level position.
        let mousex = d3.mouse(vis.node())[0] - lineStart;
        let index = Math.round(mousex / this.scale(i));
        if (index < 0) {
            index = 0;
        } else if (index >= this.levelsInfo[i].files.length) {
            index = this.levelsInfo[i].files.length - 1;
        }
        let fileNum = this.levelsInfo[i].files[index];
        let meta = data.Files[fileNum];

        // Find the start and end index of the tables
        // that overlap with filenum.
        let overlapInfo = "";
        for (let j = 1; j < this.levelsInfo.length; j++) {
            if (this.levelsInfo[i].files.length === 0) {
                continue;
            }
            let indicator = vis.select("g.clip" + this.levelsInfo[j].levelClass + " rect.indicator");
            indicator
                .attr("fill", "black")
                .attr("opacity", 0.3)
                .attr("y", this.levelsInfo[j].offset - this.levelsInfo[j].height)
                .attr("height", this.levelsInfo[j].height);
            if (j === i) {
                continue;
            }
            let fileNums = this.levelsInfo[j].files;
            for (let k in fileNums) {
                let other = data.Files[fileNums[k]];
                if (other.Largest < meta.Smallest) {
                    continue;
                }
                let s = this.scale(j);
                let t = k;
                for (; k < fileNums.length; k++) {
                    let other = data.Files[fileNums[k]];
                    if (other.Smallest >= meta.Largest) {
                        break;
                    }
                }
                if (k === t) {
                    indicator.attr("x", lineStart + s * t).attr("width", s);
                } else {
                    indicator
                        .attr("x", lineStart + s * t)
                        .attr("width", Math.max(0.5, s * (k - t)));
                }
                if (i + 1 === j && k > t) {
                    let overlapSize = this.levelsInfo[j].files
                        .slice(t, k)
                        .reduce((sum, elem) => sum + data.Files[elem].Size, 0);

                    overlapInfo =
                        " overlaps " +
                        (k - t) +
                        " @ " +
                        this.levelsInfo[j].levelString +
                        " (" +
                        humanize(overlapSize) +
                        ")";
                }
                break;
            }
        }

        reason.text(
            "[" +
                this.levelsInfo[i].levelString +
                " " +
                fileNum +
                " (" +
                humanize(data.Files[fileNum].Size) +
                ")" +
                overlapInfo +
                " <" +
                data.Keys[data.Files[fileNum].Smallest].Pretty +
                ", " +
                data.Keys[data.Files[fileNum].Largest].Pretty +
                ">" +
                "]"
        );

        vis
            .select("g.clip" + this.levelsInfo[i].levelClass + " rect.indicator")
            .attr("x", lineStart + this.scale(i) * index)
            .attr("width", 1);
    },

    // Recalculate structures related to the page width.
    updateSize: function() {
        let svg = d3.select("#slider").html("");

        let margin = { right: 10, left: 10 };

        let width = styleWidth(d3.select("#slider")) - margin.left - margin.right,
            height = styleHeight(svg);

        sliderX = d3
            .scaleLinear()
            .domain([0, data.Edits.length - 1])
            .range([0, width])
            .clamp(true);

        // Used only to generate offset ticks for slider.
        // sliderX is used to index into the data.Edits array (0-indexed).
        offsetSliderX = d3
          .scaleLinear()
          .domain([data.StartEdit, data.StartEdit + data.Edits.length - 1])
          .range([0, width]);

        let slider = svg
            .append("g")
            .attr("class", "slider")
            .attr("transform", "translate(" + margin.left + "," + height / 2 + ")");

        slider
            .append("line")
            .attr("class", "track")
            .attr("x1", sliderX.range()[0])
            .attr("x2", sliderX.range()[1])
            .select(function() {
                return this.parentNode.appendChild(this.cloneNode(true));
            })
            .attr("class", "track-inset")
            .select(function() {
                return this.parentNode.appendChild(this.cloneNode(true));
            })
            .attr("class", "track-overlay")
            .call(
                d3
                    .drag()
                    .on("start.interrupt", function() {
                        slider.interrupt();
                    })
                    .on("start drag", function() {
                        version.set(Math.round(sliderX.invert(d3.event.x)));
                    })
            );

        slider
            .insert("g", ".track-overlay")
            .attr("class", "ticks")
            .attr("transform", "translate(0," + 18 + ")")
            .selectAll("text")
            .data(offsetSliderX.ticks(10))
            .enter()
            .append("text")
            .attr("x", offsetSliderX)
            .attr("text-anchor", "middle")
            .text(function(d) {
                return d;
            });

        sliderHandle = slider
            .insert("circle", ".track-overlay")
            .attr("class", "handle")
            .attr("r", 9)
            .attr("cx", sliderX(version.index));

        levelWidth = styleWidth(vis) - 10 - lineStart;
        let lineEnd = lineStart + levelWidth;

        vis
            .selectAll("line")
            .data(this.levelsInfo)
            .attr("x2", lineEnd)
            .enter()
            .append("line")
            .attr("x1", lineStart)
            .attr("x2", lineEnd)
            .attr("y1", d => d.offset)
            .attr("y2", d => d.offset)
            .attr("stroke", "#ddd");

        vis
            .selectAll("defs clipPath rect")
            .data(this.levelsInfo)
            .attr("width", lineEnd - lineStart)
            .enter()
            .append("defs")
            .append("clipPath")
            .attr("id", d => d.levelClass)
            .append("rect")
            .attr("x", lineStart)
            .attr("y", d => d.offset - d.height)
            .attr("width", lineEnd - lineStart)
            .attr("height", d => d.height);
    },
};

window.onload = function() {
    version.init();
    version.updateSize();
    version.set(0);
};

window.addEventListener("resize", function() {
    version.updateSize();
    version.render();
});

let timer;

function startPlayback(increment) {
    timer = d3.timer(function() {
        let lastIndex = version.index;
        version.set(version.index + increment);
        if (lastIndex == version.index) {
            timer.stop();
            timer = null;
        }
    });
}

function stopPlayback() {
    if (timer == null) {
        return false;
    }
    timer.stop();
    timer = null;
    return true;
}

document.addEventListener("keydown", function(e) {
    switch (e.keyCode) {
        case 37: // left arrow
            stopPlayback();
            version.set(version.index - (e.shiftKey ? 10 : 1));
            return;
        case 39: // right arrow
            stopPlayback();
            version.set(version.index + (e.shiftKey ? 10 : 1));
            return;
        case 32: // space
            if (stopPlayback()) {
                return;
            }
            startPlayback(1);
            return;
    }
});

index.on("input", function() {
    if (!isNaN(+this.value)) {
        const val = Number(this.value) - data.StartEdit;
        if (val >= 0) {
            version.set(val);
        }
    }
});
