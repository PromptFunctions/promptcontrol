# Beta Contract

<!-- <scml> -->
<!-- <constants> -->
<pre>
BETA_ROOT = "/workspace/beta"
BETA_INPUT = "/workspace/input.txt"
</pre>
<!-- </constants> -->

<!-- <section name="READS" data-type="read-list"> -->
## READS
  - ${BETA_INPUT}
  <!-- <section name="CONFIG" data-type="read-list"> -->
  - /workspace/config.yaml
  <!-- </section> -->
<!-- </section> -->

<!-- <section name="WRITES" data-type="file-list" data-policy="write"> -->
## WRITES
  - ${BETA_ROOT}/out/report.txt
  <!-- <section name="ARCHIVE" data-type="file-list" data-policy="write"> -->
  - ${BETA_ROOT}/archive/report.tar
  <!-- </section> -->
<!-- </section> -->
<!-- </scml> -->
