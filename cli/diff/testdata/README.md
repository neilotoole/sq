# `diff/testdata`

This dir contains testdata for the `sq` diff tests. It also contains scripts that execute
against GNU [`diff`](https://www.gnu.org/software/diffutils/manual/html_node/index.html)
to compare the output of `sq` with the `diff`'s output.

## Reference

- https://en.wikipedia.org/wiki/Diff#Unified_format
- https://www.gnu.org/software/diffutils/manual/html_node/Hunks.html
- https://www.gnu.org/software/diffutils/manual/html_node/Sections.html
- https://www.cloudbees.com/blog/git-diff-a-complete-comparison-tutorial-for-git


## Sakila diff fixtures

[`sakila_a.db`](sakila_a.db) and [`sakila_b.db`](sakila_b.db) are two SQLite Sakila
databases that differ from each other, used by `cli/diff/diff_test.go` to exercise
`sq diff` end to end (schema diffs, the exit-code-1-on-difference contract, and
output-format rejection).

[`sakila_a.actor.txt`](sakila_a.actor.txt) and [`sakila_b.actor.txt`](sakila_b.actor.txt)
are the `actor` table from each, rendered as text, so the same difference can be fed to
GNU `diff` for comparison against `sq`'s own output. The visible difference is in the
second row: `WAHLBERG` in `a`, `BERGER` in `b`.

## diffdirs

[`diffdirs.sh`](diffdirs.sh) executes `diff` against two dirs.

![Output from terminal](diffdirs.png)

Below is that same diff output, but inside a markdown `diff` block.

```diff
$ ./diffdirs.sh
diff '--color=auto' -U3 -r ./diffdirs/a/hiawatha.txt ./diffdirs/b/hiawatha.txt
--- ./diffdirs/a/hiawatha.txt	2024-02-16 11:34:47.496687882 -0700
+++ ./diffdirs/b/hiawatha.txt	2024-02-16 11:37:26.940711827 -0700
@@ -7,7 +7,7 @@
 With their frequent repetitions,
 And their wild reverberations
 As of thunder in the mountains?
-  I should answer, I should tell you,
+  I should answer, I should tell you all,
 "From the forests and the prairies,
 From the great lakes of the Northland,
 From the land of the Ojibways,
diff '--color=auto' -U3 -r ./diffdirs/a/kubla.txt ./diffdirs/b/kubla.txt
--- ./diffdirs/a/kubla.txt	2024-02-16 11:32:29.305630663 -0700
+++ ./diffdirs/b/kubla.txt	2024-02-16 11:33:24.119176094 -0700
@@ -4,7 +4,7 @@
 Through caverns measureless to man
    Down to a sunless sea.
 So twice five miles of fertile ground
-With walls and towers were girdled round;
+With walls and towers were girdled round and round;
 And there were gardens bright with sinuous rills,
 Where blossomed many an incense-bearing tree;
 And here were forests ancient as the hills,
```

## Hunk section

Here's a hunk section in a markdown `diff` block (presumably you're viewing this on GitHub).

```diff
@@ -4,7 +4,7 @@ Here's some context I've added.
 Through caverns measureless to man
    Down to a sunless sea.
 So twice five miles of fertile ground
-With walls and towers were girdled round;
+With walls and towers were girdled round and round;
```

## GitHub comps

### Split view

![kubla_github_split.png](kubla_github_split.png)

### Unified view

![kubla_github_unified.png](kubla_github_unified.png)
