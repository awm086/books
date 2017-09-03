function showSearchPage() {
  $("#search-page").show()
  $("#view-page").hide()
}

function showViewPage() {
  $("#search-page").hide()
  $("#view-page").show()
}

function deleteBook(pk) {
  $.ajax({
    url: "/books/?pk=" + +pk,
    method: "DELETE",
    success: function () {
      $("#book-row-" + pk).remove();
    }
  })
}

function submitSearch() {
  console.log('submitting search');
  $.ajax({
    url: "/search",
    method: "POST",
    data: $("#search-form").serialize(),
    success: function (rawData) {
      console.log(rawData)
      var parsed = JSON.parse(rawData);

      if (!parsed) return;
      var elem = $('#search-results');
      elem.empty();
      parsed.forEach(function (result) {
        console.log(result)
        var row = $("<tr><td>" + result.Title + "</td><td>"
          + result.Author + "</td><td>"
          + result.Year + "</td><td>"
          + result.ID + "</td>"
        );
        elem.append(row);
        row.on('click', function () {
          $.ajax({
            url: "/books?id=" + result.ID,
            method: "PUT",
            success: function (data) {
              var book = JSON.parse(data);
              if (!book) return;
              $("#view-results").append("<tr id='book-row-" + book.PK + "'><td>" + book.Title + "</td><td>" + book.Author + "</td><td>" + book.Classification + "</td><td><button class=btn-delete onclick='deleteBook(" + book.PK + ")' </td>Delete</tr>")
            }
          })
        });
      });
    }
  });
  return false
}